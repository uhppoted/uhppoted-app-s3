package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
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
	"sort"
	"strings"
	"text/template"
	"time"
)

var report = `ACL DIFF REPORT {{ .DateTime }}
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

var PUT_ACL = PutACL{
	config: DEFAULT_CONFIG,
	debug:  false,
}

type PutACL struct {
	config string
	url    string
	debug  bool
}

func (l *PutACL) Name() string {
	return "put-acl"
}

func (l *PutACL) FlagSet() *flag.FlagSet {
	flagset := flag.NewFlagSet("put-acl", flag.ExitOnError)

	flagset.StringVar(&l.url, "url", l.url, "The S3 URL for the ACL file")
	flagset.BoolVar(&l.debug, "debug", l.debug, "Enables debugging information")

	return flagset
}

func (l *PutACL) Description() string {
	return fmt.Sprintf("Fetches an access control list from S3 and loads it to the configured controllers")
}

func (l *PutACL) Usage() string {
	return "put-acl [--debug] --url <S3 URL>"
}

func (l *PutACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s put-acl --url <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Fetches the ACL file stored at the pre-signed S3 URL and loads it to the controllers configured in:\n\n")
	fmt.Printf("       %s\n", l.config)
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Println("      url   (required) Pre-signed S3 URL for the ACL file")
	fmt.Println("      debug (optional) Displays verbose debug information")
	fmt.Println()
}

func (l *PutACL) Execute(ctx context.Context) error {
	if strings.TrimSpace(l.url) == "" {
		return fmt.Errorf("put-acl requires a pre-signed S3 URL in the command options")
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

	return l.execute(&u, uri.String(), devices)
}

func (l *PutACL) execute(u device.IDevice, uri string, devices []*uhppote.Device) error {
	log.Printf("Fetching ACL from %v\n", uri)

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

	log.Printf("Fetched  ACL from %v (%d bytes)\n", uri, N)

	f.Close()

	var buffer bytes.Buffer

	untar(f.Name(), &buffer)

	log.Printf("Extracted ACL from %v\n", uri)

	m, err := acl.ParseTSV(&buffer, devices)
	if err != nil {
		return err
	}

	for k, l := range m {
		log.Printf("%v  %v records", k, len(l))
	}

	current, err := acl.GetACL(u, devices)
	if err != nil {
		return err
	}

	diff, err := acl.Compare(m, current)
	if err != nil {
		return err
	}

	t, err := template.New("report").Parse(report)
	if err != nil {
		return err
	}

	rpt := struct {
		DateTime types.DateTime
		Diffs    map[uint32]acl.Diff
	}{
		DateTime: types.DateTime(time.Now()),
		Diffs:    diff,
	}

	err = t.Execute(os.Stdout, rpt)
	if err != nil {
		return err
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
