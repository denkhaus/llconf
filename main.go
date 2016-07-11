package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/cmd"
	"github.com/juju/errors"
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
			Value:  "localhost",
		},
		cli.IntFlag{
			Name:   "port, P",
			Usage:  "the port used for communication",
			EnvVar: "LLCONF_PORT",
			Value:  9954,
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
	}

	app.Commands = []cli.Command{
		cmd.NewRunCommand(),
		cmd.NewServeCommand(),
		cmd.NewTestCommand(),
		cmd.NewWatchCommand(),
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("execution error: %s\n",
			errors.ErrorStack(err))
		os.Exit(1)
	}
}
