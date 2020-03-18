package main

import (
	"context"
	"fmt"
	"github.com/uhppoted/uhppoted-acl-s3/commands"
	"github.com/uhppoted/uhppoted-api/command"
	"os"
)

var cli = []uhppoted.Command{
	&commands.LOAD_ACL,
	&commands.GET_ACL,
	&uhppoted.VERSION,
}

var help = uhppoted.NewHelp(commands.SERVICE, cli, nil)

func main() {
	cmd, err := uhppoted.Parse(cli, nil, help)
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
