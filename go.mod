module github.com/uhppoted/uhppoted-acl-s3

go 1.14

require (
	github.com/uhppoted/uhppote-core v0.5.2-0.20200317195304-e87a1741fa95
	github.com/uhppoted/uhppoted-api v0.5.2-0.20200318192830-d343ef7b7170
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae
)

replace (
	github.com/uhppoted/uhppote-core => ../uhppote-core
	github.com/uhppoted/uhppoted-api => ../uhppoted-api
)
