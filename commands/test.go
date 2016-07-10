package commands

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/juju/errors"
)

func NewTestCommand(logger *logrus.Logger) cli.Command {
	return cli.Command{
		Name: "test",
		Action: func(ctx *cli.Context) error {
			rCtx, err := NewRunCtx(ctx, logger, true)
			if err != nil {
				return errors.Annotate(err, "new run context")
			}

			if _, err := rCtx.compilePromise(); err != nil {
				return errors.Annotate(err, "compile promise")
			}

			rCtx.AppLogger.Info("test successful")
			return nil
		},
	}
}
