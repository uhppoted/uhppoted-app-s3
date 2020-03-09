package commands

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var LOAD_ACL = LoadACL{
	conf: DEFAULT_CONFIG,
}

type LoadACL struct {
	conf string
	url  string
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
	fmt.Printf("    Fetches the ACL file stored at the S3 URL and loads it to the controllers configured in:\n\n")
	fmt.Printf("       %s\n", l.conf)
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

	fmt.Printf(" ... fetching ACL from %v\n", uri)

	response, err := http.Get(uri.String())
	if err != nil {
		return err
	}

	defer response.Body.Close()

	f, err := ioutil.TempFile(os.TempDir(), "ubc-acl-*")
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	N, err := io.Copy(f, response.Body)
	if err != nil {
		return err
	}

	fmt.Printf(" ... fetched  ACL from %v (%d bytes)\n", uri, N)

	f.Close()

	untar(f.Name())

	return nil
}

func untar(filepath string) error {
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

		fmt.Printf(">>> %s:\n", header.Name)
		println("---")
		if _, err := io.Copy(os.Stdout, tr); err != nil {
			return err
		}

		fmt.Println()
		println("---")
	}

	return nil
}
