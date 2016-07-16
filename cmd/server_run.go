package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/juju/errors"
)

func newServerRunCommand() cli.Command {
	return cli.Command{
		Name: "run",
		Action: func(ctx *cli.Context) error {
			rCtx, err := context.New(ctx, false, false)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}
			defer rCtx.Close()

			if err := rCtx.CreateServer(); err != nil {
				return errors.Annotate(err, "create server")
			}

			return nil
		},
	}
}
