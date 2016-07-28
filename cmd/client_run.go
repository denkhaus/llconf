package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

func newClientRunCommand() cli.Command {
	return cli.Command{
		Name: "run",
		Action: func(ctx *cli.Context) error {
			if err := clientRun(ctx); err != nil {
				logging.Logger.Error(err)
			}
			return nil
		},
	}
}

func clientRun(ctx *cli.Context) error {
	logging.Logger.Info("exec: client run")

	rCtx, err := context.New(ctx, true, true)
	if err != nil {
		return errors.Annotate(err, "new run context")
	}
	defer rCtx.Close()

	if err := rCtx.CreateClient(); err != nil {
		return errors.Annotate(err, "create client")
	}

	tree, err := rCtx.CompilePromise()
	if err != nil {
		return errors.Annotate(err, "compile promise")
	}

	if err := rCtx.SendPromise(tree); err != nil {
		return errors.Annotate(err, "send promise")
	}

	return nil
}
