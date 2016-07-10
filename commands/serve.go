package commands

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/juju/errors"
)

func NewServeCommand(logger *logrus.Logger) cli.Command {

	return cli.Command{
		Name: "serve",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "input-folder, i",
				Usage:  "the folder containing input files",
				EnvVar: "LLCONF_INPUT_FOLDER",
			},
			cli.StringFlag{
				Name:   "runlog-path, r",
				Usage:  "path to the runlog",
				EnvVar: "LLCONF_RUNLOG",
			},
		},
		Action: func(ctx *cli.Context) error {
			rCtx, err := NewRunCtx(ctx, logger, false)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}

			if err := rCtx.createServer(); err != nil {
				return errors.Annotate(err, "create server")
			}

			return nil
		},
	}
}
