package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

func newClientTestCommand() cli.Command {
	return cli.Command{
		Name: "test",
		Action: func(ctx *cli.Context) error {
			rCtx, err := context.New(ctx, true, true)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}
			defer rCtx.Close()

			if _, err := rCtx.CompilePromise(); err != nil {
				return errors.Annotate(err, "compile promise")
			}

			logging.Logger.Info("test successful")
			return nil
		},
	}
}
