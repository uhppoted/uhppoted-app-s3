package main

import (
	"context"
	"fmt"
	"github.com/uhppoted/uhppoted-acl-s3/commands"
	"github.com/uhppoted/uhppoted-api/command"
	"os"
)

var cli = []uhppoted.Command{
	&uhppoted.VERSION,
}

var help = uhppoted.NewHelp(commands.SERVICE, cli, &uhppoted.VERSION)

func main() {
	cmd, err := uhppoted.Parse(cli, &uhppoted.VERSION, help)
	if err != nil {
		fmt.Printf("\nError parsing command line: %v\n\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err = cmd.Execute(ctx); err != nil {
		fmt.Printf("\nERROR: %v\n\n", err)
		os.Exit(1)
	}
}
