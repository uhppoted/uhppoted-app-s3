package commands

import (
	"path/filepath"
)

var DEFAULT_CONFIG = filepath.Join(workdir(), "uhppoted.conf")
var DEFAULT_WORKDIR = workdir()
var DEFAULT_KEYSDIR = filepath.Join(workdir(), "acl", "keys")
var DEFAULT_CREDENTIALS = filepath.Join(workdir(), ".aws", "credentials")
var DEFAULT_REGION = "us-east-1"
