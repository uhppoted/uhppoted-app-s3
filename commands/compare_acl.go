package commands

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	syslog "log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-lib/acl"
	"github.com/uhppoted/uhppoted-lib/config"
	"github.com/uhppoted/uhppoted-lib/eventlog"

	"github.com/uhppoted/uhppoted-app-s3/log"
)

var CompareACLCmd = CompareACL{
	config:      config.DefaultConfig,
	keysdir:     DEFAULT_KEYSDIR,
	keyfile:     DEFAULT_KEYFILE,
	credentials: DEFAULT_CREDENTIALS,
	profile:     DEFAULT_PROFILE,
	region:      DEFAULT_REGION,
	withPIN:     false,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	noverify:    false,
	nolog:       false,
	debug:       false,
	template: `ACL DIFF REPORT {{ .DateTime }}
{{range $id,$value := .Diffs}}
  DEVICE {{ $id }}{{if or $value.Updated $value.Added $value.Deleted}}{{else}} OK{{end}}{{if $value.Updated}}
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
	withPIN     bool
	logFile     string
	logFileSize int
	template    string
	noverify    bool
	nolog       bool
	debug       bool
}

func (cmd *CompareACL) Name() string {
	return "compare-acl"
}

func (cmd *CompareACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("compare-acl", flag.ExitOnError)

	flagset.StringVar(&cmd.acl, "acl", cmd.acl, "The URL for the authoritative ACL file")
	flagset.StringVar(&cmd.rpt, "report", cmd.rpt, "The URL for the uploaded report file")
	flagset.StringVar(&cmd.credentials, "credentials", cmd.credentials, "AWS credentials file")
	flagset.StringVar(&cmd.profile, "profile", cmd.profile, "AWS credentials file profile (defaults to 'default')")
	flagset.StringVar(&cmd.region, "region", cmd.region, "AWS region for S3 (defaults to us-east-1)")
	flagset.BoolVar(&cmd.withPIN, "with-pin", cmd.withPIN, "Includes the card keypad PIN codes in the ACL comparison")
	flagset.StringVar(&cmd.keysdir, "keys", cmd.keysdir, "Sets the directory to search for RSA signing keys. Key files are expected to be named '<uname>.pub'")
	flagset.StringVar(&cmd.keyfile, "key", cmd.keyfile, "RSA signing key")
	flagset.BoolVar(&cmd.noverify, "no-verify", cmd.noverify, "Disables verification of the downloaded ACL RSA signature")
	flagset.BoolVar(&cmd.nolog, "no-log", cmd.nolog, "Writes log messages to stdout rather than a rotatable log file")

	return flagset
}

func (cmd *CompareACL) Description() string {
	return "Retrieves the ACL from the configured controllers, compares it to the authoritative ACL at the provided URL and uploads the report to S3"
}

func (cmd *CompareACL) Usage() string {
	return "compare-acl <S3 URL>"
}

func (cmd *CompareACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s [--debug] [--config <file>] compare--acl --acl <URL> --report <URL> [--credentials <file>] [--profile <file>] [--region <region>] [--keys <dir>] [--key <file>] [--no-verify] [--no-log]\n", APP)
	fmt.Println()
	fmt.Println("    Retrieves the ACL from the controllers configured in the configuration file, compares it to the authoritative ACL")
	fmt.Println("    fetched from the --acl URL and uploads the comparison report to the --report URL.")
	fmt.Println()

	helpOptions(cmd.FlagSet())
	fmt.Println()
}

func (cmd *CompareACL) Execute(args ...interface{}) error {
	options := args[0].(*Options)

	cmd.config = options.Config
	cmd.debug = options.Debug

	// ... check parameters
	if strings.TrimSpace(cmd.acl) == "" {
		return fmt.Errorf("compare-acl requires a URL for the authoritative ACL file")
	}

	if strings.TrimSpace(cmd.rpt) == "" {
		return fmt.Errorf("compare-acl requires a URL to upload the compare report")
	}

	uri, err := url.Parse(cmd.acl)
	if err != nil {
		return fmt.Errorf("invalid ACL file URL '%s' (%w)", cmd.acl, err)
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

	return cmd.execute(u, uri.String(), devices)
}

func (cmd *CompareACL) execute(u uhppote.IUHPPOTE, uri string, devices []uhppote.Device) error {
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

	list, warnings, err := acl.ParseTSV(bytes.NewReader(tsv), devices, false)
	if err != nil {
		return err
	}

	for _, w := range warnings {
		log.Warnf("%v", w)
	}

	for k, l := range list {
		log.Infof("%v  Retrieved %v records", k, len(l))
	}

	current, errors := acl.GetACL(u, devices)
	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}

	compare := func(current acl.ACL, list acl.ACL) (map[uint32]acl.Diff, error) {
		if cmd.withPIN {
			return acl.CompareWithPIN(current, list)
		} else {
			return acl.Compare(current, list)
		}
	}

	if diff, err := compare(current, list); err != nil {
		return err
	} else {
		for k, v := range diff {
			log.Infof("%v  SUMMARY  same:%v  different:%v  missing:%v  extraneous:%v", k, len(v.Unchanged), len(v.Updated), len(v.Added), len(v.Deleted))
		}

		return cmd.upload(diff)
	}
}

func (cmd *CompareACL) fetchHTTP(url string) ([]byte, error) {
	return fetchHTTP(url)
}

func (cmd *CompareACL) fetchS3(url string) ([]byte, error) {
	return fetchS3(url, cmd.credentials, cmd.profile, cmd.region)
}

func (cmd *CompareACL) fetchFile(url string) ([]byte, error) {
	return fetchFile(url)
}

func (cmd *CompareACL) storeHTTP(url string, r io.Reader) error {
	return storeHTTP(url, r)
}

func (cmd *CompareACL) storeS3(uri string, r io.Reader) error {
	return storeS3(uri, cmd.credentials, cmd.profile, cmd.region, r)
}

func (cmd *CompareACL) storeFile(url string, r io.Reader) error {
	return storeFile(url, r)
}

func (cmd *CompareACL) upload(diff map[uint32]acl.Diff) error {
	log.Infof("Uploading ACL 'diff' report")

	var w strings.Builder

	if err := report(diff, cmd.template, &w); err != nil {
		return err
	}

	filename := time.Now().Format("acl-2006-01-02T150405.rpt")
	rpt := []byte(w.String())
	signature, err := sign(rpt, cmd.keyfile)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	var files = map[string][]byte{
		filename:    rpt,
		"signature": signature,
	}

	x := targz
	if strings.HasSuffix(cmd.rpt, ".zip") {
		x = zipf
	}

	if err := x(files, &b); err != nil {
		return err
	}

	log.Infof("tar'd report (%v bytes) and signature (%v bytes): %v bytes", len(rpt), len(signature), b.Len())

	f := cmd.storeHTTP
	if strings.HasPrefix(cmd.rpt, "s3://") {
		f = cmd.storeS3
	} else if strings.HasPrefix(cmd.rpt, "file://") {
		f = cmd.storeFile
	}

	if err := f(cmd.rpt, bytes.NewReader(b.Bytes())); err != nil {
		return err
	}

	log.Infof("Uploaded to %v", cmd.rpt)

	return nil
}
