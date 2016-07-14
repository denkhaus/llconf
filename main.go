package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/cmd"
	"github.com/denkhaus/llconf/logging"
)

func main() {
	app := cli.NewApp()
	app.Name = "llconf"
	app.Usage = "A batch execution tool for remote or local use."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "the host the promise is executed on",
			EnvVar: "LLCONF_HOST",
			Value:  "0.0.0.0",
		},
		cli.IntFlag{
			Name:   "port, P",
			Usage:  "the port used for communication",
			EnvVar: "LLCONF_PORT",
			Value:  9954,
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
		cli.StringFlag{
			Name:   "runlog-path, r",
			Usage:  "path to runlog",
			EnvVar: "LLCONF_RUNLOG",
		},
		cli.BoolFlag{
			Name:   "syslog, s",
			Usage:  "output to syslog",
			EnvVar: "LLCONF_SYSLOG",
		},
	}

	app.Commands = []cli.Command{
		cmd.NewClientCommand(),
		cmd.NewServerCommand(),
	}

	if err := app.Run(os.Args); err != nil {
		logging.Logger.Error(err)
		os.Exit(1)
	}
}
