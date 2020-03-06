package commands

import (
	"context"
	"flag"
	"fmt"
)

var STORE_ACL = StoreACL{
	conf: DEFAULT_CONFIG,
}

type StoreACL struct {
	conf string
}

func (s *StoreACL) Name() string {
	return "store-acl"
}

func (s *StoreACL) FlagSet() *flag.FlagSet {
	return flag.NewFlagSet("store-acl", flag.ExitOnError)
}

func (s *StoreACL) Description() string {
	return fmt.Sprintf("Retrieves the ACL from the configured controllers and uploads to S3")
}

func (s *StoreACL) Usage() string {
	return "store-acs <S3 URL>"
}

func (s *StoreACL) Help() {
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

func (s *StoreACL) Execute(ctx context.Context) error {
	return fmt.Errorf("** NOT IMPLEMENTED **")
}
