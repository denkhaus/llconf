package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/commands"
)

var (
	logger *logrus.Logger
)

func init() {
	logger = logrus.New()
	logger.Level = logrus.DebugLevel
	logger.Out = os.Stdout
}

func main() {
	app := cli.NewApp()
	app.Name = "llconf"
	app.Usage = "A lisp like configuration management tool"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "promise, p",
			Usage:  "the promise that will be used as root",
			EnvVar: "LLCONF_PROMISE",
			Value:  "done",
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name: "eval",
			Action: func(ctx *cli.Context) {
				commands.Eval(ctx, logger)
			},
		},
		cli.Command{
			Name: "serve",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:   "interval, n",
					Usage:  "set the minium time between promise-tree evaluation",
					EnvVar: "LLCONF_INTERVAL",
					Value:  300,
				},
				cli.BoolFlag{
					Name:   "verbose, v",
					Usage:  "enable verbose output",
					EnvVar: "LLCONF_VERBOSE",
				},
				cli.StringFlag{
					Name:   "input-folder, i",
					Usage:  "the folder containing input files",
					EnvVar: "LLCONF_INPUT_FOLDER",
				},
				cli.BoolFlag{
					Name:   "syslog, s",
					Usage:  "output to syslog",
					EnvVar: "LLCONF_SYSLOG",
				},
				cli.StringFlag{
					Name:   "runlog, r",
					Usage:  "path to the runlog",
					EnvVar: "LLCONF_RUNLOG",
				},
			},
			Action: func(ctx *cli.Context) {
				commands.Serve(ctx, logger)
			},
		},
	}

	app.Run(os.Args)
}
