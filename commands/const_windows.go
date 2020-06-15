package commands

import (
	"path/filepath"
)

var DEFAULT_WORKDIR = workdir()
var DEFAULT_KEYSDIR = filepath.Join(workdir(), "acl", "keys")
var DEFAULT_KEYFILE = filepath.Join(workdir(), "acl", "keys", "uhppoted")
var DEFAULT_CREDENTIALS = ""
var DEFAULT_PROFILE = ""
var DEFAULT_REGION = ""
var DEFAULT_LOGFILE = filepath.Join(workdir(), "logs", "uhppoted-app-s3.log")
var DEFAULT_LOGFILESIZE = 10
