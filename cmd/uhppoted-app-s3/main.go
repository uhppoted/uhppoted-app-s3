package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/uhppoted/uhppote-core/uhppote"
	"github.com/uhppoted/uhppoted-api/command"
	"github.com/uhppoted/uhppoted-app-s3/commands"
)

var cli = []uhppoted.CommandV{
	&commands.LOAD_ACL,
	&commands.STORE_ACL,
	&commands.COMPARE_ACL,
	&uhppoted.VersionV{
		Application: commands.SERVICE,
		Version:     uhppote.VERSION,
	},
}

var help = uhppoted.NewHelpV(commands.SERVICE, cli, nil)

func main() {
	flag.Parse()

	cmd, err := uhppoted.ParseV(cli, nil, help)
	if err != nil {
		fmt.Printf("\nError parsing command line: %v\n\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if cmd == nil {
		help.Execute(ctx)
		os.Exit(1)
	}

	if err = cmd.Execute(ctx); err != nil {
		fmt.Printf("\n   ERROR: %v\n\n", err)
		os.Exit(1)
	}
}
