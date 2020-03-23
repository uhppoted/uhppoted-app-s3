package commands

import (
	"path/filepath"
)

var DEFAULT_CONFIG = filepath.Join(workdir(), "uhppoted.conf")
var DEFAULT_WORKDIR = workdir()
var DEFAULT_KEYSDIR = filepath.Join(workdir(), "acl", "keys")
var DEFAULT_CREDENTIALS = filepath.Join(workdir(), ".aws", "credentials")
var DEFAULT_REGION = "us-east-1"
var DEFAULT_LOGFILE = filepath.Join(workdir(), "logs", "uhppoted-acl-s3.log")
var DEFAULT_LOGFILESIZE = 10
