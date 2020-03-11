package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-api/acl"
	"github.com/uhppoted/uhppoted-api/config"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

var LOAD_ACL = LoadACL{
	config: DEFAULT_CONFIG,
}

type LoadACL struct {
	config string
	url    string
}

func (l *LoadACL) Name() string {
	return "load-acl"
}

func (l *LoadACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("load-acl", flag.ExitOnError)

	flagset.StringVar(&l.url, "url", l.url, "The S3 URL for the ACL file")

	return flagset
}

func (l *LoadACL) Description() string {
	return fmt.Sprintf("Fetches an access control list from S3 and loads it to the configured controllers")
}

func (l *LoadACL) Usage() string {
	return "load-acl --url <S3 URL>"
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
	fmt.Println("      url  (required) Pre-signed S3 URL for the ACL file")
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

	devices := []*uhppote.Device{}
	for _, id := range keys {
		d := conf.Devices[id]
		devices = append(devices, uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors))
	}

	return l.execute(uri.String(), devices)
}

func (l *LoadACL) execute(uri string, devices []*uhppote.Device) error {
	log.Printf(" ... fetching ACL from %v\n", uri)

	response, err := http.Get(uri)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	f, err := ioutil.TempFile(os.TempDir(), "uhppoted-acl-*")
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	N, err := io.Copy(f, response.Body)
	if err != nil {
		return err
	}

	log.Printf(" ... fetched  ACL from %v (%d bytes)\n", uri, N)

	f.Close()

	var buffer bytes.Buffer

	untar(f.Name(), &buffer)

	log.Printf(" ... untar'd  ACL from %v\n", uri)

	m, err := acl.ParseTSV(&buffer, devices)
	if err != nil {
		return err
	}

	log.Printf(" ... parsed ACL: %v\n", m)

	for k, l := range m {
		fmt.Printf(">> DEBUG: %v\n", k)
		for cn, c := range l {
			fmt.Printf("          %v %v\n", cn, c)
		}
	}

	return nil
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
