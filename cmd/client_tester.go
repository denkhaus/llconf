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
			if err := clientTest(ctx); err != nil {
				logging.Logger.Error(err)
			}
			return nil
		},
	}
}

func clientTest(ctx *cli.Context) error {
	logging.Logger.Infof("%s exec: client test", ctx.App.Version)

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
}
