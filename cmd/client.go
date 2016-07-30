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
				Usage:  "enable verbose output",
				EnvVar: "LLCONF_VERBOSE",
			},
			cli.BoolFlag{
				Name:   "debug",
				Usage:  "enable debug output",
				EnvVar: "LLCONF_DEBUG",
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
