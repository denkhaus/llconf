package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/cmd"
	"github.com/denkhaus/llconf/logging"
)

func main() {
	app := cli.NewApp()
	app.Name = "llconf"
	app.EnableBashCompletion = true
	app.Version = fmt.Sprintf("%s-%s", AppVersion, Revision)
	app.Usage = "A batch execution tool for remote and local use."

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "the host the promise is executed on",
			EnvVar: "LLCONF_HOST",
			Value:  "localhost",
		},
		cli.IntFlag{
			Name:   "port, P",
			Usage:  "the port used for communication",
			EnvVar: "LLCONF_PORT",
			Value:  9954,
		},
		cli.BoolFlag{
			Name:  "revision",
			Usage: "Print revision",
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
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "enable debug output in client and server mode",
			EnvVar: "LLCONF_DEBUG",
		},
	}

	app.Commands = []cli.Command{
		cmd.NewClientCommand(),
		cmd.NewServerCommand(),
	}

	app.Action = func(ctx *cli.Context) error {
		if ctx.GlobalBool("revision") {
			fmt.Println(Revision)
		} else {
			cli.ShowAppHelp(ctx)
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logging.Logger.Error(err)
		os.Exit(1)
	}
}
