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
	"github.com/uhppoted/uhppoted-api/acl"
	"github.com/uhppoted/uhppoted-api/config"
	"io"
	"io/ioutil"
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
	credentials: DEFAULT_CREDENTIALS,
	region:      DEFAULT_REGION,
	noreport:    false,
	debug:       false,
}

type LoadACL struct {
	config      string
	workdir     string
	credentials string
	region      string
	url         string
	noreport    bool
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
	flagset.BoolVar(&l.noreport, "no-report", l.noreport, "Disables ACL 'diff' report")
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
	fmt.Printf("  Usage: %s load-acl --url <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Fetches the ACL file stored at the pre-signed S3 URL and loads it to the controllers configured in:\n\n")
	fmt.Printf("       %s\n", l.config)
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Println("      url         (required) Pre-signed S3 URL for the ACL file")
	fmt.Printf("      credentials (optional) File path for the AWS credentials (defaults to %s)\n", l.credentials)
	fmt.Printf("      region      (optional) AWS region for S3 (defaults to %s)\n", l.region)
	fmt.Println("      no-report   (optional) Disables creation of the 'diff' between the current and fetched ACL's")
	fmt.Println("      debug       (optional) Displays verbose debug information")
	fmt.Println()
}

func (l *LoadACL) Execute(ctx context.Context) error {
	if strings.TrimSpace(l.url) == "" {
		return fmt.Errorf("load-acl requires a pre-signed S3 URL in the command options")
	}

	uri, err := url.Parse(l.url)
	if err != nil {
		return fmt.Errorf("Invalid pre-signed S3 URL '%s' (%w)", l.url, err)
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

	logger := log.New(os.Stdout, "ACL ", log.LstdFlags|log.LUTC|log.Lmsgprefix)

	return l.execute(&u, uri.String(), devices, logger)
}

func (l *LoadACL) execute(u device.IDevice, uri string, devices []*uhppote.Device, log *log.Logger) error {
	log.Printf("Fetching ACL from %v", uri)

	var f *os.File
	var err error

	if strings.HasPrefix(uri, "s3://") {
		f, err = l.fetchS3(uri, log)
	} else {
		f, err = l.fetchHTTP(uri, log)
	}

	if err != nil {
		return err
	} else if f == nil {
		return fmt.Errorf("'fetch' returned invalid file handle")
	}

	defer os.Remove(f.Name())

	var buffer bytes.Buffer

	untar(f.Name(), &buffer)

	log.Printf("Extracted ACL from %v", uri)

	list, err := acl.ParseTSV(&buffer, devices)
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

	err = acl.PutACL(u, list)
	if err != nil {
		return err
	}

	return nil
}

func (l *LoadACL) fetchHTTP(uri string, log *log.Logger) (*os.File, error) {
	response, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	f, err := ioutil.TempFile(os.TempDir(), "uhppoted-acl-*")
	if err != nil {
		return nil, err
	}

	N, err := io.Copy(f, response.Body)
	if err != nil {
		os.Remove(f.Name())
		return nil, err
	}

	log.Printf("Fetched  ACL from %v (%d bytes)", uri, N)

	f.Close()

	return f, nil
}

func (l *LoadACL) fetchS3(uri string, log *log.Logger) (*os.File, error) {
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

	f, err := ioutil.TempFile(os.TempDir(), "uhppoted-acl-*")
	if err != nil {
		return nil, err
	}

	N, err := s3manager.NewDownloader(s).Download(f, &object)
	if err != nil {
		os.Remove(f.Name())
		return nil, err
	}

	log.Printf("Fetched  ACL from %v (%d bytes)", uri, N)

	f.Close()

	return f, nil
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

func untar(filepath string, w io.Writer) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeReg:
			if _, err := io.Copy(w, tr); err != nil {
				return err
			}
		}
	}

	return nil
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
               {{end}}{{end}}{{end}}`

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
