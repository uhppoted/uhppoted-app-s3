package commands

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/uhppoted/uhppote-core/types"
	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-app-s3/auth"
	"github.com/uhppoted/uhppoted-lib/acl"
	"github.com/uhppoted/uhppoted-lib/config"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"text/template"
	"time"
)

type Report struct {
	DateTime *types.DateTime
	Diffs    map[uint32]acl.Diff
}

func getDevices(conf *config.Config, debug bool) (uhppote.IUHPPOTE, []uhppote.Device) {
	bind, broadcast, listen := config.DefaultIpAddresses()

	if conf.BindAddress != nil {
		bind = *conf.BindAddress
	}

	if conf.BroadcastAddress != nil {
		broadcast = *conf.BroadcastAddress
	}

	if conf.ListenAddress != nil {
		listen = *conf.ListenAddress
	}

	devices := []uhppote.Device{}
	for s, d := range conf.Devices {
		// ... because d is *Device and all devices end up with the same info if you don't make a manual copy
		name := d.Name
		deviceID := s
		address := d.Address
		doors := d.Doors

		if device := uhppote.NewDevice(name, deviceID, address, doors); device != nil {
			devices = append(devices, *device)
		}
	}

	u := uhppote.NewUHPPOTE(bind, broadcast, listen, 5*time.Second, devices, debug)

	return u, devices
}

func fetchHTTP(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	var b bytes.Buffer
	if _, err = io.Copy(&b, response.Body); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func fetchS3(url, config, profile, region string) ([]byte, error) {
	match := regexp.MustCompile("^s3://(.*?)/(.*)").FindStringSubmatch(url)
	if len(match) != 3 {
		return nil, fmt.Errorf("Invalid S3 URI (%s)", url)
	}

	bucket := match[1]
	key := match[2]
	object := s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	cfg := aws.NewConfig().
		WithCredentials(credentials.NewSharedCredentials(config, profile)).
		WithRegion(region)

	ss := session.Must(session.NewSession(cfg))

	buffer := make([]byte, 1024)
	b := aws.NewWriteAtBuffer(buffer)
	if _, err := s3manager.NewDownloader(ss).Download(b, &object); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func fetchFile(url string) ([]byte, error) {
	match := regexp.MustCompile("^file://(.*)").FindStringSubmatch(url)
	if len(match) != 2 {
		return nil, fmt.Errorf("Invalid file URI (%s)", url)
	}

	return ioutil.ReadFile(match[1])
}

func storeHTTP(uri string, r io.Reader) error {
	rq, err := http.NewRequest("PUT", uri, r)
	if err != nil {
		return err
	}

	rq.Header.Set("Content-Type", "binary/octet-stream")

	response, err := http.DefaultClient.Do(rq)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	return nil
}

func storeS3(uri, config, profile, region string, r io.Reader) error {
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

	cfg := aws.NewConfig().
		WithCredentials(credentials.NewSharedCredentials(config, profile)).
		WithRegion(region)

	ss := session.Must(session.NewSession(cfg))
	_, err := s3manager.NewUploader(ss).Upload(&object)
	if err != nil {
		return err
	}

	return nil
}

func storeFile(url string, r io.Reader) error {
	match := regexp.MustCompile("^file://(.*)").FindStringSubmatch(url)
	if len(match) != 2 {
		return fmt.Errorf("Invalid file URI (%s)", url)
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(match[1], b, 0660)
}

func targz(files map[string][]byte, w io.Writer) error {
	var b bytes.Buffer

	tw := tar.NewWriter(&b)
	for filename, body := range files {
		header := &tar.Header{
			Name:  filename,
			Mode:  0660,
			Size:  int64(len(body)),
			Uname: "uhppoted",
			Gname: "uhppoted",
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if _, err := tw.Write([]byte(body)); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}

	gz := gzip.NewWriter(w)

	gz.Name = fmt.Sprintf("uhppoted-%s.tar.gz", time.Now().Format("2006-01-02T150405"))
	gz.ModTime = time.Now()
	gz.Comment = ""

	_, err := gz.Write(b.Bytes())
	if err != nil {
		return err
	}

	return gz.Close()
}

func untar(r io.Reader) (map[string][]byte, string, error) {
	files := map[string][]byte{}
	uname := ""

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, "", err
	}

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, "", err
		}

		switch header.Typeflag {
		case tar.TypeReg:
			if filepath.Ext(header.Name) == ".acl" {
				if _, ok := files["ACL"]; ok {
					return nil, "", fmt.Errorf("Multiple ACL files in tar.gz")
				}

				var buffer bytes.Buffer
				if _, err := io.Copy(&buffer, tr); err != nil {
					return nil, "", err
				}

				files["ACL"] = buffer.Bytes()
				uname = header.Uname
			}

			if header.Name == "signature" {
				if _, ok := files["signature"]; ok {
					return nil, "", fmt.Errorf("Multiple signature files in tar.gz")
				}

				var buffer bytes.Buffer
				if _, err := io.Copy(&buffer, tr); err != nil {
					return nil, "", err
				}

				files["signature"] = buffer.Bytes()
			}
		}
	}

	if _, ok := files["ACL"]; !ok {
		return nil, "", fmt.Errorf("ACL file missing from tar.gz")
	}

	if _, ok := files["signature"]; !ok {
		return nil, "", fmt.Errorf("'signature' file missing from tar.gz")
	}

	return files, uname, nil
}

func zipf(files map[string][]byte, w io.Writer) error {
	zw := zip.NewWriter(w)
	for filename, body := range files {
		if f, err := zw.Create(filename); err != nil {
			return err
		} else if _, err = f.Write([]byte(body)); err != nil {
			return err
		}
	}

	return zw.Close()
}

func unzip(r io.Reader) (map[string][]byte, string, error) {
	files := map[string][]byte{}
	uname := ""

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, "", err
	}

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, "", err
	}

	for _, f := range zr.File {
		if filepath.Ext(f.Name) == ".acl" {
			if _, ok := files["ACL"]; ok {
				return nil, "", fmt.Errorf("Multiple ACL files in tar.gz")
			}

			rc, err := f.Open()
			if err != nil {
				return nil, "", err
			}

			var buffer bytes.Buffer
			if _, err := io.Copy(&buffer, rc); err != nil {
				return nil, "", err
			}

			files["ACL"] = buffer.Bytes()
			uname = f.Comment
			rc.Close()
		}

		if f.Name == "signature" {
			if _, ok := files["signature"]; ok {
				return nil, "", fmt.Errorf("Multiple signature files in tar.gz")
			}

			rc, err := f.Open()
			if err != nil {
				return nil, "", err
			}

			var buffer bytes.Buffer
			if _, err := io.Copy(&buffer, rc); err != nil {
				return nil, "", err
			}

			files["signature"] = buffer.Bytes()
			rc.Close()
		}
	}

	if _, ok := files["ACL"]; !ok {
		return nil, "", fmt.Errorf("ACL file missing from tar.gz")
	}

	if _, ok := files["signature"]; !ok {
		return nil, "", fmt.Errorf("'signature' file missing from tar.gz")
	}

	return files, uname, nil
}

func sign(acl []byte, keyfile string) ([]byte, error) {
	return auth.Sign(acl, keyfile)
}

func verify(uname string, acl, signature []byte, dir string) error {
	return auth.Verify(uname, acl, signature, dir)
}

func report(diff map[uint32]acl.Diff, format string, w io.Writer) error {
	t, err := template.New("report").Parse(format)
	if err != nil {
		return err
	}

	timestamp := types.DateTime(time.Now())

	rpt := Report{
		DateTime: &timestamp,
		Diffs:    diff,
	}

	return t.Execute(w, rpt)
}
