package commands

import (
	"archive/tar"
	"bufio"
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
	"github.com/uhppoted/uhppoted-acl-s3/auth"
	"github.com/uhppoted/uhppoted-api/acl"
	"github.com/uhppoted/uhppoted-api/config"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"text/template"
	"time"
)

type Report struct {
	DateTime types.DateTime
	Diffs    map[uint32]acl.Diff
}

type File struct {
	Name string
	Body []byte
}

func getDevices(conf *config.Config, debug bool) (uhppote.UHPPOTE, []*uhppote.Device) {
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
		Debug:            debug,
	}

	devices := []*uhppote.Device{}
	for _, id := range keys {
		d := conf.Devices[id]
		u.Devices[id] = uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors)
		devices = append(devices, uhppote.NewDevice(id, d.Address, d.Rollover, d.Doors))
	}

	return u, devices
}

func fetchHTTP(url string, log *log.Logger) ([]byte, error) {
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

func storeHTTP(uri string, r io.Reader) error {
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

	return nil
}

func fetchS3(uri, awsconfig, region string, log *log.Logger) ([]byte, error) {
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

	credentials, err := getAWSCredentials(awsconfig)
	if err != nil {
		return nil, err
	}

	cfg := aws.NewConfig().
		WithCredentials(credentials).
		WithRegion(region)

	ss := session.Must(session.NewSession(cfg))

	buffer := make([]byte, 1024)
	b := aws.NewWriteAtBuffer(buffer)
	N, err := s3manager.NewDownloader(ss).Download(b, &object)
	if err != nil {
		return nil, err
	}

	log.Printf("Fetched ACL from %v (%d bytes)", uri, N)

	return b.Bytes(), nil
}

func storeS3(uri, awsconfig, region string, r io.Reader) error {
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

	credentials, err := getAWSCredentials(awsconfig)
	if err != nil {
		return err
	}

	cfg := aws.NewConfig().
		WithCredentials(credentials).
		WithRegion(region)

	ss := session.Must(session.NewSession(cfg))

	_, err = s3manager.NewUploader(ss).Upload(&object)
	if err != nil {
		return err
	}

	return nil
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

func targz(files []File, w io.Writer) error {
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

	gz.Name = fmt.Sprintf("uhppoted-%s.tar.gz", time.Now().Format("2006-01-02T150405"))
	gz.ModTime = time.Now()
	gz.Comment = ""

	_, err := gz.Write(b.Bytes())
	if err != nil {
		return err
	}

	return gz.Close()
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

func sign(acl []byte, keyfile string) ([]byte, error) {
	return auth.Sign(acl, keyfile)
}

func verify(uname string, acl, signature []byte, dir string) error {
	return auth.Verify(uname, acl, signature, dir)
}

func report(current, list acl.ACL, format string, w io.Writer) error {
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

	return t.Execute(w, rpt)
}
