package commands

import (
	"context"
	"flag"
	"fmt"
)

var GET_ACL = GetACL{
	conf: DEFAULT_CONFIG,
}

type GetACL struct {
	conf string
}

func (s *GetACL) Name() string {
	return "get-acl"
}

func (s *GetACL) FlagSet() *flag.FlagSet {
	return flag.NewFlagSet("get-acl", flag.ExitOnError)
}

func (s *GetACL) Description() string {
	return fmt.Sprintf("Retrieves the ACL from the configured controllers and uploads to S3")
}

func (s *GetACL) Usage() string {
	return "store-acs <S3 URL>"
}

func (s *GetACL) Help() {
	fmt.Println()
	fmt.Printf("  Usage: %s store-acs <url>\n", SERVICE)
	fmt.Println()
	fmt.Printf("    Retrieves the ACL from the controllers configured in:\n\n")
	fmt.Printf("       %s\n\n", s.conf)
	fmt.Printf("    and stores it to the provided S3 URL\n")
	fmt.Println()
	fmt.Println("    Options:")
	fmt.Println()
	fmt.Println("      url  (required) Pre-signed S3 URL for the ACL file")
	fmt.Println()
}

func (s *GetACL) Execute(ctx context.Context) error {
	return fmt.Errorf("** NOT IMPLEMENTED **")
}
