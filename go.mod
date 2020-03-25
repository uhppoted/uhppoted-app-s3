module github.com/uhppoted/uhppoted-acl-s3

go 1.14

require (
	github.com/aws/aws-sdk-go v1.29.27
	github.com/uhppoted/uhppote-core v0.5.2-0.20200325185445-fd835261278f
	github.com/uhppoted/uhppoted-api v0.5.2-0.20200325185547-a1183e9869a8
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae
)

replace (
	github.com/uhppoted/uhppote-core => ../uhppote-core
	github.com/uhppoted/uhppoted-api => ../uhppoted-api
)
