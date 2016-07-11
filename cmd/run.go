package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/juju/errors"
)

func NewRunCommand() cli.Command {

	return cli.Command{
		Name: "run",
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
			rCtx, err := NewRunCtx(ctx, true)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}

			if err := rCtx.createClient(); err != nil {
				return errors.Annotate(err, "create client")
			}

			tree, err := rCtx.compilePromise()
			if err != nil {
				return errors.Annotate(err, "compile promise")
			}

			if err := rCtx.sendPromise(tree); err != nil {
				return errors.Annotate(err, "send promise")
			}

			return nil
		},
	}
}
