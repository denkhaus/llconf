package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

func NewTestCommand() cli.Command {
	return cli.Command{
		Name: "test",
		Action: func(ctx *cli.Context) error {
			rCtx, err := NewRunCtx(ctx, true)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}

			if _, err := rCtx.compilePromise(); err != nil {
				return errors.Annotate(err, "compile promise")
			}

			logging.Logger.Info("test successful")
			return nil
		},
	}
}
