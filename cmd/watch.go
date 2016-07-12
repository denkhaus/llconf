package cmd

import (
	"time"

	"github.com/codegangsta/cli"
	"github.com/juju/errors"
)

func NewWatchCommand() cli.Command {
	return cli.Command{
		Name: "watch",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:   "interval, n",
				Usage:  "set the minium time between promise-tree evaluation",
				EnvVar: "LLCONF_INTERVAL",
				Value:  300,
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
				Name:   "runlog-path, r",
				Usage:  "path to the runlog",
				EnvVar: "LLCONF_RUNLOG",
			},
		},
		Action: func(ctx *cli.Context) error {
			rCtx, err := NewRunCtx(ctx, true)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}
			if err := rCtx.setupLogging(); err != nil {
				return errors.Annotate(err, "setup logging")
			}

			quit := make(chan int)

			for {
				go func(q chan int) {
					time.Sleep(time.Duration(rCtx.Interval) * time.Second)
					q <- 0
				}(quit)

				tree, err := rCtx.compilePromise()
				if err != nil {
					return errors.Annotate(err, "compile promise")
				}

				if tree == nil {
					return errors.New("could not find any valid promises")
				}

				if err := rCtx.execPromise(tree, rCtx.Verbose); err != nil {
					return errors.Annotate(err, "exec promise")
				}

				<-quit
			}
			return nil
		},
	}
}
