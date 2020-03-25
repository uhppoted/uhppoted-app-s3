package commands

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/uhppoted/uhppote-core/device"
	"github.com/uhppoted/uhppote-core/types"
	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-acl-s3/auth"
	"github.com/uhppoted/uhppoted-api/acl"
	"github.com/uhppoted/uhppoted-api/config"
	"github.com/uhppoted/uhppoted-api/eventlog"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

var LOAD_ACL = LoadACL{
	config:      DEFAULT_CONFIG,
	workdir:     DEFAULT_WORKDIR,
	keysdir:     DEFAULT_KEYSDIR,
	credentials: DEFAULT_CREDENTIALS,
	region:      DEFAULT_REGION,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	noreport:    false,
	noverify:    false,
	nolog:       false,
	debug:       false,
}

type LoadACL struct {
	url         string
	config      string
	workdir     string
	keysdir     string
	credentials string
	region      string
	logFile     string
	logFileSize int
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

	flagset.StringVar(&l.url, "url", l.url, "The S3 URL for the ACL file")
	flagset.StringVar(&l.credentials, "credentials", l.credentials, "Filepath for the AWS credentials")
	flagset.StringVar(&l.region, "region", l.region, "The AWS region for S3 (defaults to us-east-1)")
	flagset.StringVar(&l.keysdir, "keys", l.keysdir, "Sets the directory to search for RSA signing keys. Key files are expected to be named '<uname>.pub'")
	flagset.StringVar(&l.config, "config", l.config, "'conf' file to use for controller identification and configuration")
	flagset.StringVar(&l.workdir, "workdir", l.workdir, "Sets the working directory for temporary files, etc")
	flagset.BoolVar(&l.noverify, "no-verify", l.noverify, "Disables verification of the ACL signature")
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
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Println("      url         (required) URL for the ACL file. S3 URL's are formatted as s3://<bucket>/<key>")
	fmt.Printf("      credentials (optional) File path for the AWS credentials for S3 URL's (defaults to %s)\n", l.credentials)
	fmt.Printf("      region      (optional) AWS region for S3 (defaults to %s)\n", l.region)
	fmt.Printf("      keys        (optional) Directory containing for RSA signing keys (defaults to %s). Key files are expected to be named '<uname>.pub", l.keysdir)
	fmt.Printf("      config      (optional) File path for the 'conf' file containing the controller configuration (defaults to %s)\n", l.config)
	fmt.Printf("      workdir     (optional) Sets the working directory for temporary files, etc (defaults to %s)\n", l.workdir)
	fmt.Printf("      no-verify   (optional) Disables verification of the ACL signature. Defaults to '%v'\n", l.noverify)
	fmt.Println("      no-report   (optional) Disables creation of the 'diff' between the current and fetched ACL's")
	fmt.Println("      no-log      (optional) Disables event logging to the uhppoted-acl-s3.log file (events are logged to stdout instead)")
	fmt.Println("      debug       (optional) Displays verbose debug information")
	fmt.Println()
}

func (l *LoadACL) Execute(ctx context.Context) error {
	if strings.TrimSpace(l.url) == "" {
		return fmt.Errorf("load-acl requires a pre-signed S3 URL in the command options")
	}

	uri, err := url.Parse(l.url)
	if err != nil {
		return fmt.Errorf("Invalid download URL '%s' (%w)", l.url, err)
	}

	conf := config.NewConfig()
	if err := conf.Load(l.config); err != nil {
		return fmt.Errorf("WARN  Could not load configuration (%v)", err)
	}

	keys := []uint32{}
	for id, _ := range conf.Devices {
		keys = append(keys, id)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	u := uhppote.UHPPOTE{
		BindAddress:      conf.BindAddress,
		BroadcastAddress: conf.BroadcastAddress,
		ListenAddress:    conf.ListenAddress,
		Devices:          make(map[uint32]*uhppote.Device),
		Debug:            l.debug,
	}

	devices := []*uhppote.Device{}
	for _, id := range keys {
		d := conf.Devices[id]
		u.Devices[id] = uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors)
		devices = append(devices, uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors))
	}

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
	}

	b, err := f(uri, log)
	if err != nil {
		return err
	}

	r := bytes.NewReader(b)
	tsv, signature, uname, err := untar(r)

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

		report(current, list, l.workdir, log)
	}

	rpt, err := acl.PutACL(u, list)
	for k, v := range rpt {
		log.Printf("%v  SUMMARY  unchanged:%v  updated:%v  added:%v  deleted:%v  failed:%v", k, v.Unchanged, v.Updated, v.Added, v.Deleted, v.Failed)
	}

	return err
}

func (l *LoadACL) fetchHTTP(uri string, log *log.Logger) ([]byte, error) {
	response, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	var b bytes.Buffer
	N, err := io.Copy(&b, response.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("Fetched  ACL from %v (%d bytes)", uri, N)

	return b.Bytes(), nil
}

func (l *LoadACL) fetchS3(uri string, log *log.Logger) ([]byte, error) {
	match := regexp.MustCompile("^s3://(.*?)/(.*)").FindStringSubmatch(uri)
	if len(match) != 3 {
		return nil, fmt.Errorf("Invalid S3 URI (%s)", uri)
	}

	bucket := match[1]
	key := match[2]
	object := s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	credentials, err := getAWSCredentials(l.credentials)
	if err != nil {
		return nil, err
	}

	cfg := aws.NewConfig().
		WithCredentials(credentials).
		WithRegion(l.region)

	s := session.Must(session.NewSession(cfg))

	buffer := make([]byte, 1024)
	b := aws.NewWriteAtBuffer(buffer)
	N, err := s3manager.NewDownloader(s).Download(b, &object)
	if err != nil {
		return nil, err
	}

	log.Printf("Fetched  ACL from %v (%d bytes)", uri, N)

	return b.Bytes(), nil
}

func getAWSCredentials(file string) (*credentials.Credentials, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	awsKeyID := ""
	awsSecret := ""
	re := regexp.MustCompile(`\s*(aws_access_key_id|aws_secret_access_key)\s*=\s*(\S+)\s*`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindSubmatch([]byte(line))
		if len(match) == 3 {
			switch string(match[1]) {
			case "aws_access_key_id":
				awsKeyID = string(match[2])
			case "aws_secret_access_key":
				awsSecret = string(match[2])
			}
		}
	}

	if awsKeyID == "" {
		return nil, fmt.Errorf("Invalid AWS credentials - missing 'aws_access_key_id'")
	}

	if awsSecret == "" {
		return nil, fmt.Errorf("Invalid AWS credentials - missing 'aws_secret_access_key'")
	}

	return credentials.NewStaticCredentials(awsKeyID, awsSecret, ""), nil
}

func untar(r io.Reader) ([]byte, []byte, string, error) {
	var acl bytes.Buffer
	var signature bytes.Buffer
	var uname = ""

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, "", err
	}

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, nil, "", err
		}

		switch header.Typeflag {
		case tar.TypeReg:
			if filepath.Ext(header.Name) == ".acl" {
				uname = header.Uname
				if _, err := io.Copy(&acl, tr); err != nil {
					return nil, nil, "", err
				}
			}

			if header.Name == "signature" {
				if _, err := io.Copy(&signature, tr); err != nil {
					return nil, nil, "", err
				}
			}
		}
	}

	return acl.Bytes(), signature.Bytes(), uname, nil
}

func verify(uname string, acl, signature []byte, dir string) error {
	return auth.Verify(uname, acl, signature, dir)
}

var format = `ACL DIFF REPORT {{ .DateTime }}
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
`

type Report struct {
	DateTime types.DateTime
	Diffs    map[uint32]acl.Diff
}

func report(current, list acl.ACL, dir string, log *log.Logger) error {
	log.Printf("Generating ACL 'diff' report")
	diff, err := acl.Compare(current, list)
	if err != nil {
		return err
	}

	t, err := template.New("report").Parse(format)
	if err != nil {
		return err
	}

	rpt := Report{
		DateTime: types.DateTime(time.Now()),
		Diffs:    diff,
	}

	filename := time.Now().Format("acl-2006-01-02T15:04:05.rpt")
	file := filepath.Join(dir, filename)
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Printf("Writing 'diff' report to %v", f.Name())

	t.Execute(os.Stdout, rpt)

	return t.Execute(f, rpt)
}
