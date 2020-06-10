package commands

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/uhppoted/uhppote-core/device"
	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-api/acl"
	"github.com/uhppoted/uhppoted-api/config"
	"github.com/uhppoted/uhppoted-api/eventlog"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var LOAD_ACL = LoadACL{
	config:      DEFAULT_CONFIG,
	workdir:     DEFAULT_WORKDIR,
	keysdir:     DEFAULT_KEYSDIR,
	credentials: DEFAULT_CREDENTIALS,
	profile:     DEFAULT_PROFILE,
	region:      DEFAULT_REGION,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	noreport:    false,
	noverify:    false,
	nolog:       false,
	debug:       false,
	template: `ACL DIFF REPORT {{ .DateTime }}
{{range $id,$value := .Diffs}}
  DEVICE {{ $id }}{{if $value.Unchanged}}
    Unchanged: {{range $value.Unchanged}}{{.}}
               {{end}}{{end}}{{if $value.Updated}}
    Updated:   {{range $value.Updated}}{{.}}
               {{end}}{{end}}{{if $value.Added}}
    Added:     {{range $value.Added}}{{.}}
               {{end}}{{end}}{{if $value.Deleted}}
    Deleted:   {{range $value.Deleted}}{{.}}
               {{end}}{{end}}{{end}}
`,
}

type LoadACL struct {
	url         string
	config      string
	workdir     string
	keysdir     string
	credentials string
	profile     string
	region      string
	logFile     string
	logFileSize int
	template    string
	noreport    bool
	noverify    bool
	nolog       bool
	debug       bool
}

func (l *LoadACL) Name() string {
	return "load-acl"
}

func (l *LoadACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("load-acl", flag.ExitOnError)

	flagset.StringVar(&l.url, "url", l.url, "The URL from which to fetch the ACL file")
	flagset.StringVar(&l.credentials, "credentials", l.credentials, "AWS credentials file")
	flagset.StringVar(&l.profile, "profile", l.profile, "AWS credentials file profile (defaults to 'default')")
	flagset.StringVar(&l.region, "region", l.region, "AWS region for S3 (defaults to us-east-1)")
	flagset.StringVar(&l.keysdir, "keys", l.keysdir, "Sets the directory to search for RSA signing keys. Key files are expected to be named '<uname>.pub'")
	flagset.StringVar(&l.config, "config", l.config, "'conf' file to use for controller identification and configuration")
	flagset.StringVar(&l.workdir, "workdir", l.workdir, "Sets the working directory for temporary files, etc")
	flagset.BoolVar(&l.noverify, "no-verify", l.noverify, "Disables verification of the downloaded ACL RSA signature")
	flagset.BoolVar(&l.noreport, "no-report", l.noreport, "Disables ACL 'diff' report")
	flagset.BoolVar(&l.nolog, "no-log", l.nolog, "Writes log messages to stdout rather than a rotatable log file")
	flagset.BoolVar(&l.debug, "debug", l.debug, "Enables debugging information")

	return flagset
}

func (l *LoadACL) Description() string {
	return fmt.Sprintf("Fetches an access control list from S3 and loads it to the configured controllers")
}

func (l *LoadACL) Usage() string {
	return "load-acl [--debug] --url <S3 URL>"
}

func (l *LoadACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s load-acl [options] --url <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Fetches the ACL file stored at the pre-signed S3 URL and loads it to the controllers configured in:\n\n")
	fmt.Printf("       %s\n", l.config)
	fmt.Println()
	fmt.Println("      url         (required) URL for the ACL file. S3 URL's are formatted as s3://<bucket>/<key>")
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Printf("      config      (optional) File path for the 'conf' file containing the controller configuration (defaults to %s)\n", l.config)
	fmt.Printf("      credentials (optional) Overrides AWS credentials file path in config\n")
	fmt.Printf("      profile     (optional) Overrides AWS credentials profile in config\n")
	fmt.Printf("      region      (optional) Overrides AWS region in config\n")
	fmt.Printf("      keys        (optional) Directory containing for RSA signing keys (defaults to %s).\n", l.keysdir)
	fmt.Printf("                             Key files are expected to be named '<uname>.pub\n")
	fmt.Printf("      workdir     (optional) Sets the working directory for temporary files, etc (defaults to %s)\n", l.workdir)
	fmt.Printf("      no-verify   (optional) Disables verification of the ACL signature. Defaults to '%v'\n", l.noverify)
	fmt.Println("      no-report   (optional) Disables creation of the 'diff' between the current and fetched ACL's")
	fmt.Println("      no-log      (optional) Disables event logging to the uhppoted-acl-s3.log file (events are logged to stdout instead)")
	fmt.Println("      debug       (optional) Displays verbose debug information")
	fmt.Println()
}

func (l *LoadACL) Execute(ctx context.Context) error {
	if strings.TrimSpace(l.url) == "" {
		return fmt.Errorf("load-acl requires a URL for the authoritative ACL file in the command options")
	}

	uri, err := url.Parse(l.url)
	if err != nil {
		return fmt.Errorf("Invalid ACL file URL '%s' (%w)", l.url, err)
	}

	conf := config.NewConfig()
	if err := conf.Load(l.config); err != nil {
		return fmt.Errorf("WARN  Could not load configuration (%v)", err)
	}

	if l.credentials == "" {
		l.credentials = conf.AWS.Credentials
	}

	if l.profile == "" {
		l.profile = conf.AWS.Profile
	}

	if l.region == "" {
		l.region = conf.AWS.Region
	}

	u, devices := getDevices(conf, l.debug)

	var logger *log.Logger
	if !l.nolog {
		events := eventlog.Ticker{Filename: l.logFile, MaxSize: l.logFileSize}
		logger = log.New(&events, "", log.Ldate|log.Ltime|log.LUTC)
	} else {
		logger = log.New(os.Stdout, "ACL ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	}

	return l.execute(&u, uri.String(), devices, logger)
}

func (l *LoadACL) execute(u device.IDevice, uri string, devices []*uhppote.Device, log *log.Logger) error {
	log.Printf("Fetching ACL from %v", uri)

	f := l.fetchHTTP
	if strings.HasPrefix(uri, "s3://") {
		f = l.fetchS3
	} else if strings.HasPrefix(uri, "file://") {
		f = l.fetchFile
	}

	b, err := f(uri)
	if err != nil {
		return err
	}

	log.Printf("Fetched ACL from %v (%d bytes)", uri, len(b))

	x := untar
	if strings.HasSuffix(uri, ".zip") {
		x = unzip
	}

	files, uname, err := x(bytes.NewReader(b))
	if err != nil {
		return err
	}

	tsv, ok := files["ACL"]
	if !ok {
		return fmt.Errorf("ACL file missing from tar.gz")
	}

	signature, ok := files["signature"]
	if !l.noverify && !ok {
		return fmt.Errorf("'signature' file missing from tar.gz")
	}

	log.Printf("Extracted ACL from %v: %v bytes, signature: %v bytes", uri, len(tsv), len(signature))

	if !l.noverify {
		if err := verify(uname, tsv, signature, l.keysdir); err != nil {
			return err
		}
	}

	list, err := acl.ParseTSV(bytes.NewReader(tsv), devices)
	if err != nil {
		return err
	}

	for k, l := range list {
		log.Printf("%v  Retrieved %v records", k, len(l))
	}

	if !l.noreport {
		current, err := acl.GetACL(u, devices)
		if err != nil {
			return err
		}

		l.report(current, list, log)
	}

	rpt, err := acl.PutACL(u, list)
	for k, v := range rpt {
		log.Printf("%v  SUMMARY  unchanged:%v  updated:%v  added:%v  deleted:%v  failed:%v", k, v.Unchanged, v.Updated, v.Added, v.Deleted, v.Failed)
	}

	return err
}

func (l *LoadACL) fetchHTTP(url string) ([]byte, error) {
	return fetchHTTP(url)
}

func (l *LoadACL) fetchS3(url string) ([]byte, error) {
	return fetchS3(url, l.credentials, l.profile, l.region)
}

func (l *LoadACL) fetchFile(url string) ([]byte, error) {
	return fetchFile(url)
}

func (l *LoadACL) report(current, list acl.ACL, log *log.Logger) error {
	log.Printf("Generating ACL 'diff' report")

	diff, err := acl.Compare(current, list)
	if err != nil {
		return err
	}

	report(diff, l.template, os.Stdout)

	filename := time.Now().Format("acl-2006-01-02T150405.rpt")
	file := filepath.Join(l.workdir, filename)
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	defer f.Close()

	log.Printf("Writing 'diff' report to %v", f.Name())

	return report(diff, l.template, f)
}
