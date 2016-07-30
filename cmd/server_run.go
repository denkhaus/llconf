package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

////////////////////////////////////////////////////////////////////////////////
func newServerRunCommand() cli.Command {
	return cli.Command{
		Name: "run",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "no-redirect",
				Usage: "do not redirect processing output to client",
			},
		},
		Action: func(ctx *cli.Context) error {
			if err := serverRun(ctx); err != nil {
				logging.Logger.Error(err)
			}
			return nil
		},
	}
}

////////////////////////////////////////////////////////////////////////////////
func serverRun(ctx *cli.Context) error {
	logging.Logger.Infof("%s exec: server run", ctx.App.Version)

	rCtx, err := context.New(ctx, false, false)
	if err != nil {
		return errors.Annotate(err, "new run context")
	}
	defer rCtx.Close()

	if err := rCtx.StartServer(); err != nil {
		return errors.Annotate(err, "start server")
	}

	return nil
}
