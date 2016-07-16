package cmd

import "github.com/codegangsta/cli"

func NewServerCommand() cli.Command {
	cd := cli.Command{
		Name: "server",
		Subcommands: cli.Commands{
			newServerRunCommand(),
			newServerCertCommand(),
		},
	}

	return cd
}
