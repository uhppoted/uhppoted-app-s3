module github.com/uhppoted/uhppoted-acl-s3

go 1.14

require (
	github.com/uhppoted/uhppote-core v0.5.2-0.20200311190222-18d08a0b976e
	github.com/uhppoted/uhppoted-api v0.5.2-0.20200312173155-7830eeaa4052
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae
)

replace (
	github.com/uhppoted/uhppote-core => ../uhppote-core
	github.com/uhppoted/uhppoted-api => ../uhppoted-api
)
