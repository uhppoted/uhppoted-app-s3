package commands

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	syslog "log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-lib/acl"
	"github.com/uhppoted/uhppoted-lib/config"
	"github.com/uhppoted/uhppoted-lib/eventlog"
	"github.com/uhppoted/uhppoted-lib/lockfile"

	"github.com/uhppoted/uhppoted-app-s3/log"
)

var StoreACLCmd = StoreACL{
	config:      config.DefaultConfig,
	workdir:     DEFAULT_WORKDIR,
	keyfile:     DEFAULT_KEYFILE,
	credentials: DEFAULT_CREDENTIALS,
	profile:     DEFAULT_PROFILE,
	region:      DEFAULT_REGION,
	withPIN:     false,
	logFile:     DEFAULT_LOGFILE,
	logFileSize: DEFAULT_LOGFILESIZE,
	nolog:       false,
	debug:       false,
}

type StoreACL struct {
	url         string
	config      string
	workdir     string
	keyfile     string
	credentials string
	profile     string
	region      string
	withPIN     bool
	logFile     string
	logFileSize int
	nosign      bool
	nolog       bool
	debug       bool
}

func (cmd *StoreACL) Name() string {
	return "store-acl"
}

func (cmd *StoreACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("store-acl", flag.ExitOnError)

	flagset.StringVar(&cmd.url, "url", cmd.url, "URL for a 'PUT' request to upload the retrieved ACL file")
	flagset.StringVar(&cmd.credentials, "credentials", cmd.credentials, "AWS credentials file")
	flagset.StringVar(&cmd.profile, "profile", cmd.profile, "AWS credentials file profile (defaults to 'default')")
	flagset.StringVar(&cmd.region, "region", cmd.region, "AWS region for S3 (defaults to us-east-1)")
	flagset.StringVar(&cmd.keyfile, "key", cmd.keyfile, "RSA signing key")
	flagset.BoolVar(&cmd.nosign, "no-sign", cmd.nosign, "Does not sign the generated report")
	flagset.BoolVar(&cmd.nolog, "no-log", cmd.nolog, "Writes log messages to stdout rather than a rotatable log file")
	flagset.BoolVar(&cmd.withPIN, "with-pin", cmd.withPIN, "Includes the card keypad PIN codes in the retrieved ACL file")
	flagset.StringVar(&cmd.workdir, "workdir", cmd.workdir, "Sets the working directory for temporary files, etc")

	return flagset
}

func (cmd *StoreACL) Description() string {
	return "Retrieves the ACL from the configured controllers and uploads to S3"
}

func (cmd *StoreACL) Usage() string {
	return "store-acl <S3 URL>"
}

func (cmd *StoreACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s [--debug] [--config <file>] store-acl --url <URL> [--credentials <file>] [--profile <file>] [--region <region>] [--key <file>] [--no-log] [--no-sign]\n", APP)
	fmt.Println()
	fmt.Println("    Retrieves the ACL from the controllers configured in the configuration file and stores it to the provided URL")
	fmt.Println()

	helpOptions(cmd.FlagSet())

	fmt.Println()
}

func (cmd *StoreACL) Execute(args ...interface{}) error {
	if strings.TrimSpace(cmd.url) == "" {
		return fmt.Errorf("store-acl requires a pre-signed S3 URL in the command options")
	}

	uri, err := url.Parse(cmd.url)
	if err != nil {
		return fmt.Errorf("invalid upload URL '%s' (%w)", cmd.url, err)
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

func (cmd *StoreACL) execute(u uhppote.IUHPPOTE, uri string, devices []uhppote.Device) error {
	log.Infof("Storing ACL to %v", uri)

	list, errors := acl.GetACL(u, devices)
	if len(errors) > 0 {
		return fmt.Errorf("%v", errors)
	}

	for k, l := range list {
		log.Infof("%v  Retrieved %v records", k, len(l))
	}

	var w strings.Builder

	makeTSV := func() error {
		if cmd.withPIN {
			return acl.MakeTSVWithPIN(list, devices, &w)
		} else {
			return acl.MakeTSV(list, devices, &w)
		}
	}

	if err := makeTSV(); err != nil {
		return err
	}

	tsv := []byte(w.String())
	files := map[string][]byte{
		"uhppoted.acl": tsv,
	}

	if !cmd.nosign {
		signature, err := sign(tsv, cmd.keyfile)
		if err != nil {
			return err
		}
		files["signature"] = signature
	}

	var b bytes.Buffer
	x := targz
	if strings.HasSuffix(uri, ".zip") {
		x = zipf
	}

	if err := x(files, &b); err != nil {
		return err
	}

	log.Infof("tar'd ACL (%v bytes) and signature (%v bytes): %v bytes", len(files["uhppoted.acl"]), len(files["signature"]), b.Len())

	f := cmd.storeHTTP
	if strings.HasPrefix(uri, "s3://") {
		f = cmd.storeS3
	} else if strings.HasPrefix(uri, "file://") {
		f = cmd.storeFile
	}

	if err := f(uri, bytes.NewReader(b.Bytes())); err != nil {
		return err
	}

	log.Infof("Stored ACL to %v", uri)

	return nil
}

func (cmd *StoreACL) storeHTTP(url string, r io.Reader) error {
	return storeHTTP(url, r)
}

func (cmd *StoreACL) storeS3(uri string, r io.Reader) error {
	return storeS3(uri, cmd.credentials, cmd.profile, cmd.region, r)
}

func (cmd *StoreACL) storeFile(url string, r io.Reader) error {
	return storeFile(url, r)
}
