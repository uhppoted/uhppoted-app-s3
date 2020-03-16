module github.com/uhppoted/uhppoted-acl-s3

go 1.14

require (
	github.com/uhppoted/uhppote-core v0.5.2-0.20200316194506-d35c6db75e7e
	github.com/uhppoted/uhppoted-api v0.5.2-0.20200316194558-981d54507c6b
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae
)

replace (
	github.com/uhppoted/uhppote-core => ../uhppote-core
	github.com/uhppoted/uhppoted-api => ../uhppoted-api
)
