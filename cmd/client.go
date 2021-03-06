package cmd

import "github.com/codegangsta/cli"

func NewClientCommand() cli.Command {
	cd := cli.Command{
		Name: "client",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "input-folder, i",
				Usage:  "the folder containing input files",
				EnvVar: "LLCONF_INPUT_FOLDER",
			},
			cli.StringFlag{
				Name:   "promise, p",
				Usage:  "the root promise name",
				EnvVar: "LLCONF_PROMISE",
				Value:  "done",
			},
			cli.BoolFlag{
				Name:   "verbose",
				Usage:  "enable verbose output in client mode and makes server response more verbose",
				EnvVar: "LLCONF_VERBOSE",
			},
		},
		Subcommands: cli.Commands{
			newClientRunCommand(),
			newClientTestCommand(),
			newClientWatchCommand(),
			newClientCertCommand(),
		},
	}

	return cd
}
