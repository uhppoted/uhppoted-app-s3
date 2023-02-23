package commands

import (
	"bytes"
	"flag"
	"fmt"
	syslog "log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-lib/acl"
	"github.com/uhppoted/uhppoted-lib/config"
	"github.com/uhppoted/uhppoted-lib/eventlog"
	"github.com/uhppoted/uhppoted-lib/lockfile"

	"github.com/uhppoted/uhppoted-app-s3/log"
)

var LoadACLCmd = LoadACL{
	config:      config.DefaultConfig,
	workdir:     DEFAULT_WORKDIR,
	keysdir:     DEFAULT_KEYSDIR,
	credentials: DEFAULT_CREDENTIALS,
	profile:     DEFAULT_PROFILE,
	region:      DEFAULT_REGION,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	dryrun:      false,
	strict:      false,
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
	dryrun      bool
	strict      bool
	noreport    bool
	noverify    bool
	nolog       bool
	debug       bool
}

func (cmd *LoadACL) Name() string {
	return "load-acl"
}

func (cmd *LoadACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("load-acl", flag.ExitOnError)

	flagset.StringVar(&cmd.url, "url", cmd.url, "The URL from which to fetch the ACL file")
	flagset.StringVar(&cmd.credentials, "credentials", cmd.credentials, "AWS credentials file")
	flagset.StringVar(&cmd.profile, "profile", cmd.profile, "AWS credentials file profile (defaults to 'default')")
	flagset.StringVar(&cmd.region, "region", cmd.region, "AWS region for S3 (defaults to us-east-1)")
	flagset.StringVar(&cmd.keysdir, "keys", cmd.keysdir, "Sets the directory to search for RSA signing keys. Key files are expected to be named '<uname>.pub'")
	flagset.StringVar(&cmd.workdir, "workdir", cmd.workdir, "Sets the working directory for temporary files, etc")
	flagset.BoolVar(&cmd.noverify, "no-verify", cmd.noverify, "Disables verification of the downloaded ACL RSA signature")
	flagset.BoolVar(&cmd.dryrun, "dry-run", cmd.dryrun, "Simulates a load-acl without making any changes to the access controllers")
	flagset.BoolVar(&cmd.strict, "strict", cmd.strict, "Fails the load if the ACL contains duplicate card numbers")
	flagset.BoolVar(&cmd.noreport, "no-report", cmd.noreport, "Disables ACL 'diff' report")
	flagset.BoolVar(&cmd.nolog, "no-log", cmd.nolog, "Writes log messages to stdout rather than a rotatable log file")

	return flagset
}

func (cmd *LoadACL) Description() string {
	return "Fetches an access control list from S3 and loads it to the configured controllers"
}

func (cmd *LoadACL) Usage() string {
	return "load-acl [--debug] --url <S3 URL>"
}

func (cmd *LoadACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s [--debug] [--config <file>] load-acl --url <URL> [--dry-run] [--credentials <file>] [--profile <file>] [--region <region>] [--keys <dir>] [--workdir <dir>] [--strict] [--no-verify] [--no-log] [--no-report]\n", APP)
	fmt.Println()
	fmt.Println("    Fetches the ACL file stored at the pre-signed S3 URL and loads it to the controllers configured in")
	fmt.Println("    the configuration file. Duplicate card numbers are ignored (or deleted if they exist) with a warning")
	fmt.Println("    unless the --strict option is specified")
	fmt.Println()

	helpOptions(cmd.FlagSet())
	fmt.Println()
}

func (cmd *LoadACL) Execute(args ...interface{}) error {
	options := args[0].(*Options)

	cmd.config = options.Config
	cmd.debug = options.Debug

	// ... check parameters
	if strings.TrimSpace(cmd.url) == "" {
		return fmt.Errorf("load-acl requires a URL for the authoritative ACL file in the command options")
	}

	uri, err := url.Parse(cmd.url)
	if err != nil {
		return fmt.Errorf("invalid ACL file URL '%s' (%w)", cmd.url, err)
	}

	conf := config.NewConfig()
	if err := conf.Load(cmd.config); err != nil {
		return fmt.Errorf("WARN  Could not load configuration (%v)", err)
	}

	if cmd.credentials == "" {
		cmd.credentials = conf.AWS.Credentials
	}

	if cmd.profile == "" {
		cmd.profile = conf.AWS.Profile
	}

	if cmd.region == "" {
		cmd.region = conf.AWS.Region
	}

	u, devices := getDevices(conf, cmd.debug)

	if !cmd.nolog {
		events := eventlog.Ticker{Filename: cmd.logFile, MaxSize: cmd.logFileSize}
		log.SetLogger(syslog.New(&events, "", syslog.Ldate|syslog.Ltime|syslog.LUTC))
	} else {
		log.SetLogger(syslog.New(os.Stdout, "ACL ", syslog.LstdFlags|syslog.LUTC|syslog.Lmsgprefix))
	}

	// ... locked?
	lockFile := config.Lockfile{
		File:   filepath.Join(cmd.workdir, "uhppoted-app-s3.lock"),
		Remove: lockfile.RemoveLockfile,
	}

	if kraken, err := lockfile.MakeLockFile(lockFile); err != nil {
		return err
	} else {
		defer func() {
			infof("Removing lockfile '%v'", lockFile.File)
			kraken.Release()
		}()
	}

	return cmd.execute(u, uri.String(), devices)
}

func (cmd *LoadACL) execute(u uhppote.IUHPPOTE, uri string, devices []uhppote.Device) error {
	log.Infof("Fetching ACL from %v", uri)

	f := cmd.fetchHTTP
	if strings.HasPrefix(uri, "s3://") {
		f = cmd.fetchS3
	} else if strings.HasPrefix(uri, "file://") {
		f = cmd.fetchFile
	}

	b, err := f(uri)
	if err != nil {
		return err
	}

	log.Infof("Fetched ACL from %v (%d bytes)", uri, len(b))

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
	if !cmd.noverify && !ok {
		return fmt.Errorf("'signature' file missing from tar.gz")
	}

	log.Infof("Extracted ACL from %v: %v bytes, signature: %v bytes", uri, len(tsv), len(signature))

	if !cmd.noverify {
		if err := verify(uname, tsv, signature, cmd.keysdir); err != nil {
			return err
		}
	}

	list, warnings, err := acl.ParseTSV(bytes.NewReader(tsv), devices, cmd.strict)
	if err != nil {
		return err
	}

	for _, w := range warnings {
		log.Warnf("%v", w)
	}

	for k, l := range list {
		log.Infof("%v  Retrieved %v records", k, len(l))
	}

	if !cmd.noreport {
		current, errors := acl.GetACL(u, devices)
		if len(errors) > 0 {
			return fmt.Errorf("%v", errors)
		}

		cmd.report(current, list)
	}

	rpt, errors := acl.PutACL(u, list, cmd.dryrun)
	for k, v := range rpt {
		log.Infof("%v  SUMMARY  unchanged:%v  updated:%v  added:%v  deleted:%v  failed:%v  errors:%v",
			k,
			len(v.Unchanged),
			len(v.Updated),
			len(v.Added),
			len(v.Deleted),
			len(v.Failed),
			len(v.Errors))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}

	return nil
}

func (cmd *LoadACL) fetchHTTP(url string) ([]byte, error) {
	return fetchHTTP(url)
}

func (cmd *LoadACL) fetchS3(url string) ([]byte, error) {
	return fetchS3(url, cmd.credentials, cmd.profile, cmd.region)
}

func (cmd *LoadACL) fetchFile(url string) ([]byte, error) {
	return fetchFile(url)
}

func (cmd *LoadACL) report(current, list acl.ACL) error {
	log.Infof("Generating ACL 'diff' report")

	diff, err := acl.Compare(current, list)
	if err != nil {
		return err
	}

	report(diff, cmd.template, os.Stdout)

	filename := time.Now().Format("acl-2006-01-02T150405.rpt")
	file := filepath.Join(cmd.workdir, filename)
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	defer f.Close()

	log.Infof("Writing 'diff' report to %v", f.Name())

	return report(diff, cmd.template, f)
}
