package commands

import (
	"context"
	"flag"
	"fmt"
)

var LOAD_ACL = LoadACL{
	conf: DEFAULT_CONFIG,
}

type LoadACL struct {
	conf string
}

func (l *LoadACL) Name() string {
	return "load-acl"
}

func (l *LoadACL) FlagSet() *flag.FlagSet {
	return flag.NewFlagSet("load-acl", flag.ExitOnError)
}

func (l *LoadACL) Description() string {
	return fmt.Sprintf("Fetches an access control list from S3 and loads it to the configured controllers")
}

func (l *LoadACL) Usage() string {
	return "load-acl <S3 URL>"
}

func (l *LoadACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s load-acl <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Fetches the ACL file stored at the S3 URL and loads it to the controllers configured in:\n\n")
	fmt.Printf("       %s\n", l.conf)
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Println("      url  (required) Pre-signed S3 URL for the ACL file")
	fmt.Println()
}

func (l *LoadACL) Execute(ctx context.Context) error {
	return fmt.Errorf("** NOT IMPLEMENTED **")
}
