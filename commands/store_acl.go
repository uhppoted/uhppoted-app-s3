package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/uhppoted/uhppote-core/device"
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
	"regexp"
	"sort"
	"strings"
	"time"
)

var STORE_ACL = StoreACL{
	config:      DEFAULT_CONFIG,
	keyfile:     DEFAULT_KEYFILE,
	credentials: DEFAULT_CREDENTIALS,
	region:      DEFAULT_REGION,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	nolog:       false,
	debug:       false,
}

type StoreACL struct {
	url         string
	config      string
	keyfile     string
	credentials string
	region      string
	logFile     string
	logFileSize int
	nolog       bool
	debug       bool
}

func (s *StoreACL) Name() string {
	return "store-acl"
}

func (s *StoreACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("store-acl", flag.ExitOnError)

	flagset.StringVar(&s.url, "url", s.url, "URL for a 'PUT' request to upload the retrieved ACL file")
	flagset.StringVar(&s.credentials, "credentials", s.credentials, "Filepath for the AWS credentials")
	flagset.StringVar(&s.region, "region", s.region, "The AWS region for S3 (defaults to us-east-1)")
	flagset.StringVar(&s.keyfile, "key", s.keyfile, "RSA signing key")
	flagset.StringVar(&s.config, "config", s.config, "'conf' file to use for controller identification and configuration")
	flagset.BoolVar(&s.nolog, "no-log", s.nolog, "Writes log messages to stdout rather than a rotatable log file")
	flagset.BoolVar(&s.debug, "debug", s.debug, "Enables debugging information")

	return flagset
}

func (s *StoreACL) Description() string {
	return fmt.Sprintf("Retrieves the ACL from the configured controllers and uploads to S3")
}

func (s *StoreACL) Usage() string {
	return "store-acs <S3 URL>"
}

func (s *StoreACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s store-acs <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Retrieves the ACL from the controllers configured in:\n\n")
	fmt.Printf("       %s\n\n", s.config)
	fmt.Printf("    and stores it to the provided S3 URL\n")
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Println("      url     (required) URL for the uploaded ACL file")
	fmt.Printf("      credentials (optional) File path for the AWS credentials for use with S3 URL's (defaults to %s)\n", s.credentials)
	fmt.Printf("      region      (optional) AWS region for S3 (defaults to %s)\n", s.region)
	fmt.Printf("      key        (optional) RSA signing keys (defaults to %s)", s.keyfile)
	fmt.Printf("      config      (optional) File path for the 'conf' file containing the controller configuration (defaults to %s)\n", s.config)
	fmt.Println("      no-log      (optional) Disables event logging to the uhppoted-acl-s3.log file (events are logged to stdout instead)")
	fmt.Println("      debug       (optional) Displays verbose debug information")
	fmt.Println()
}

func (s *StoreACL) Execute(ctx context.Context) error {
	if strings.TrimSpace(s.url) == "" {
		return fmt.Errorf("store-acl requires a pre-signed S3 URL in the command options")
	}

	uri, err := url.Parse(s.url)
	if err != nil {
		return fmt.Errorf("Invalid upload URL '%s' (%w)", s.url, err)
	}

	conf := config.NewConfig()
	if err := conf.Load(s.config); err != nil {
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
		Debug:            s.debug,
	}

	devices := []*uhppote.Device{}
	for _, id := range keys {
		d := conf.Devices[id]
		u.Devices[id] = uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors)
		devices = append(devices, uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors))
	}

	var logger *log.Logger
	if !s.nolog {
		events := eventlog.Ticker{Filename: s.logFile, MaxSize: s.logFileSize}
		logger = log.New(&events, "", log.Ldate|log.Ltime|log.LUTC)
	} else {
		logger = log.New(os.Stdout, "ACL ", log.LstdFlags|log.LUTC|log.Lmsgprefix)
	}

	return s.execute(&u, uri.String(), devices, logger)
}

func (s *StoreACL) execute(u device.IDevice, uri string, devices []*uhppote.Device, log *log.Logger) error {
	log.Printf("Storing ACL to %v", uri)

	list, err := acl.GetACL(u, devices)
	if err != nil {
		return err
	}

	for k, l := range list {
		log.Printf("%v  Retrieved %v records", k, len(l))
	}

	var w strings.Builder
	err = acl.MakeTSV(list, devices, &w)
	if err != nil {
		return err
	}

	tsv := []byte(w.String())
	signature, err := sign(tsv, s.keyfile)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	if err := targz(tsv, signature, &b); err != nil {
		return err
	}

	log.Printf("tar'd ACL (%v bytes) and signature (%v bytes): %v bytes", len(tsv), len(signature), b.Len())

	f := s.storeHTTP
	if strings.HasPrefix(uri, "s3://") {
		f = s.storeS3
	}

	return f(uri, bytes.NewReader(b.Bytes()), log)
}

func sign(acl []byte, keyfile string) ([]byte, error) {
	return auth.Sign(acl, keyfile)
}

func targz(acl, signature []byte, w io.Writer) error {
	var files = []struct {
		Name string
		Body []byte
	}{
		{"uhppoted.acl", acl},
		{"signature", signature},
	}

	var b bytes.Buffer

	tw := tar.NewWriter(&b)
	for _, file := range files {
		header := &tar.Header{
			Name:  file.Name,
			Mode:  0600,
			Size:  int64(len(file.Body)),
			Uname: "uhppoted",
			Gname: "uhppoted",
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}

	gz := gzip.NewWriter(w)

	gz.Name = fmt.Sprintf("uhppoted-%s.tar.gz", time.Now().Format("2006-01-02 15:04:05"))
	gz.ModTime = time.Now()
	gz.Comment = ""

	_, err := gz.Write(b.Bytes())
	if err != nil {
		return err
	}

	return gz.Close()
}

func (s *StoreACL) storeHTTP(uri string, r io.Reader, log *log.Logger) error {
	rq, err := http.NewRequest("PUT", "http://localhost:8080/upload", r)
	if err != nil {
		return err
	}

	rq.Header.Set("Content-Type", "binary/octet-stream")

	response, err := http.DefaultClient.Do(rq)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	log.Printf("Uploaded to %v", uri)

	return nil
}

func (s *StoreACL) storeS3(uri string, r io.Reader, log *log.Logger) error {
	match := regexp.MustCompile("^s3://(.*?)/(.*)").FindStringSubmatch(uri)
	if len(match) != 3 {
		return fmt.Errorf("Invalid S3 URI (%s)", uri)
	}

	bucket := match[1]
	key := match[2]

	object := s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   r,
	}

	credentials, err := getAWSCredentials(s.credentials)
	if err != nil {
		return err
	}

	cfg := aws.NewConfig().
		WithCredentials(credentials).
		WithRegion(s.region)

	ss := session.Must(session.NewSession(cfg))

	_, err := s3manager.NewUploader(ss).Upload(&object)
	if err != nil {
		return err
	}

	log.Printf("Stored ACL to %v", uri)

	return nil
}
