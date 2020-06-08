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
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

var COMPARE_ACL = CompareACL{
	config:      DEFAULT_CONFIG,
	keysdir:     DEFAULT_KEYSDIR,
	keyfile:     DEFAULT_KEYFILE,
	credentials: DEFAULT_CREDENTIALS,
	profile:     DEFAULT_PROFILE,
	region:      DEFAULT_REGION,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	noverify:    false,
	nolog:       false,
	debug:       false,
	template: `ACL DIFF REPORT {{ .DateTime }}
{{range $id,$value := .Diffs}}
  DEVICE {{ $id }}{{if $value.Unchanged}}
    Incorrect:  {{range $value.Updated}}{{.}}
                {{end}}{{end}}{{if $value.Added}}
    Missing:    {{range $value.Added}}{{.}}
                {{end}}{{end}}{{if $value.Deleted}}
    Unexpected: {{range $value.Deleted}}{{.}}
                {{end}}{{end}}{{end}}
`,
}

type CompareACL struct {
	acl         string
	rpt         string
	config      string
	keysdir     string
	keyfile     string
	credentials string
	profile     string
	region      string
	logFile     string
	logFileSize int
	template    string
	noverify    bool
	nolog       bool
	debug       bool
}

func (c *CompareACL) Name() string {
	return "compare-acl"
}

func (c *CompareACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("compare-acl", flag.ExitOnError)

	flagset.StringVar(&c.acl, "acl", c.acl, "The URL for the authoritative ACL file")
	flagset.StringVar(&c.rpt, "report", c.rpt, "The URL for the uploaded report file")
	flagset.StringVar(&c.credentials, "credentials", c.credentials, "AWS credentials file")
	flagset.StringVar(&c.profile, "profile", c.profile, "AWS credentials file profile (defaults to 'default')")
	flagset.StringVar(&c.region, "region", c.region, "AWS region for S3 (defaults to us-east-1)")
	flagset.StringVar(&c.keysdir, "keys", c.keysdir, "Sets the directory to search for RSA signing keys. Key files are expected to be named '<uname>.pub'")
	flagset.StringVar(&c.keyfile, "key", c.keyfile, "RSA signing key")
	flagset.StringVar(&c.config, "config", c.config, "'conf' file to use for controller identification and configuration")
	flagset.BoolVar(&c.noverify, "no-verify", c.noverify, "Disables verification of the downloaded ACL RSA signature")
	flagset.BoolVar(&c.nolog, "no-log", c.nolog, "Writes log messages to stdout rather than a rotatable log file")
	flagset.BoolVar(&c.debug, "debug", c.debug, "Enables debugging information")

	return flagset
}

func (c *CompareACL) Description() string {
	return fmt.Sprintf("Retrieves the ACL from the configured controllers and uploads to S3")
}

func (c *CompareACL) Usage() string {
	return "compare-acl <S3 URL>"
}

func (c *CompareACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s compare-acl [options] --acl <url> --report <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Retrieves the ACL from the controllers configured in:\n\n")
	fmt.Printf("       %s\n\n", c.config)
	fmt.Printf("    and compares it to the ACL at the provided URL and stores a report at the report URL\n")
	fmt.Println()
	fmt.Println("      acl         (required) URL from which to fetch the ACL file. S3 URL's are formatted as s3://<bucket>/<key>")
	fmt.Println("      report      (optional) URL to which to store the report file. S3 URL's are formatted as s3://<bucket>/<key>")
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Printf("      credentials (optional) File path for the AWS credentials for use with S3 URL's (defaults to %s)\n", c.credentials)
	fmt.Printf("      profile     (optional) Profile in AWS credentials file for use with S3 URL's (defaults to %s)\n", c.credentials)
	fmt.Printf("      region      (optional) AWS region for S3 (defaults to %s)\n", c.region)
	fmt.Printf("      keys        (optional) Directory containing for RSA signing keys (defaults to %s). Key files are expected to be named '<uname>.pub", c.keysdir)
	fmt.Printf("      key         (optional) RSA key used to sign the retrieved ACL (defaults to %s)", c.keyfile)
	fmt.Printf("      config      (optional) File path for the 'conf' file containing the controller configuration (defaults to %s)\n", c.config)
	fmt.Printf("      no-verify   (optional) Disables verification of the ACL signature. Defaults to '%v'\n", c.noverify)
	fmt.Println("      no-log      (optional) Disables event logging to the uhppoted-acl-s3.log file (events are logged to stdout instead)")
	fmt.Println("      debug       (optional) Displays verbose debug information")
	fmt.Println()
}

func (c *CompareACL) Execute(ctx context.Context) error {
	if strings.TrimSpace(c.acl) == "" {
		return fmt.Errorf("compare-acl requires a URL for the authoritative ACL file")
	}

	if strings.TrimSpace(c.rpt) == "" {
		return fmt.Errorf("compare-acl requires a URL to upload the compare report")
	}

	uri, err := url.Parse(c.acl)
	if err != nil {
		return fmt.Errorf("Invalid ACL file URL '%s' (%w)", c.acl, err)
	}

	conf := config.NewConfig()
	if err := conf.Load(c.config); err != nil {
		return fmt.Errorf("WARN  Could not load configuration (%v)", err)
	}

	u, devices := getDevices(conf, c.debug)

	var logger *log.Logger
	if !c.nolog {
		events := eventlog.Ticker{Filename: c.logFile, MaxSize: c.logFileSize}
		logger = log.New(&events, "", log.Ldate|log.Ltime|log.LUTC)
	} else {
		logger = log.New(os.Stdout, "ACL ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	}

	return c.execute(&u, uri.String(), devices, logger)
}

func (c *CompareACL) execute(u device.IDevice, uri string, devices []*uhppote.Device, log *log.Logger) error {
	log.Printf("Fetching ACL from %v", uri)

	f := c.fetchHTTP
	if strings.HasPrefix(uri, "s3://") {
		f = c.fetchS3
	} else if strings.HasPrefix(uri, "file://") {
		f = c.fetchFile
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
	if !c.noverify && !ok {
		return fmt.Errorf("'signature' file missing from tar.gz")
	}

	log.Printf("Extracted ACL from %v: %v bytes, signature: %v bytes", uri, len(tsv), len(signature))

	if !c.noverify {
		if err := verify(uname, tsv, signature, c.keysdir); err != nil {
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

	current, err := acl.GetACL(u, devices)
	if err != nil {
		return err
	}

	diff, err := acl.Compare(current, list)
	if err != nil {
		return err
	}

	for k, v := range diff {
		log.Printf("%v  SUMMARY  same:%v  different:%v  missing:%v  extraneous:%v", k, len(v.Unchanged), len(v.Updated), len(v.Added), len(v.Deleted))
	}

	if err := c.upload(diff, log); err != nil {
		return err
	}

	return nil
}

func (c *CompareACL) fetchHTTP(url string) ([]byte, error) {
	return fetchHTTP(url)
}

func (c *CompareACL) fetchS3(url string) ([]byte, error) {
	return fetchS3(url, c.credentials, c.profile, c.region)
}

func (c *CompareACL) fetchFile(url string) ([]byte, error) {
	return fetchFile(url)
}

func (c *CompareACL) storeHTTP(url string, r io.Reader) error {
	return storeHTTP(url, r)
}

func (c *CompareACL) storeS3(uri string, r io.Reader) error {
	return storeS3(uri, c.credentials, c.profile, c.region, r)
}

func (c *CompareACL) storeFile(url string, r io.Reader) error {
	return storeFile(url, r)
}

func (c *CompareACL) upload(diff map[uint32]acl.Diff, log *log.Logger) error {
	log.Printf("Uploading ACL 'diff' report")

	var w strings.Builder

	if err := report(diff, c.template, &w); err != nil {
		return err
	}

	filename := time.Now().Format("acl-2006-01-02T150405.rpt")
	rpt := []byte(w.String())
	signature, err := sign(rpt, c.keyfile)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	var files = map[string][]byte{
		filename:    rpt,
		"signature": signature,
	}

	x := targz
	if strings.HasSuffix(c.rpt, ".zip") {
		x = zipf
	}

	if err := x(files, &b); err != nil {
		return err
	}

	log.Printf("tar'd report (%v bytes) and signature (%v bytes): %v bytes", len(rpt), len(signature), b.Len())

	f := c.storeHTTP
	if strings.HasPrefix(c.rpt, "s3://") {
		f = c.storeS3
	} else if strings.HasPrefix(c.rpt, "file://") {
		f = c.storeFile
	}

	if err := f(c.rpt, bytes.NewReader(b.Bytes())); err != nil {
		return err
	}

	log.Printf("Uploaded to %v", c.rpt)

	return nil
}
