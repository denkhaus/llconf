package commands

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func Run(ctx *cli.Context, logger *logrus.Logger) {
	rCtx, err := NewRunCtx(ctx, logger)
	if err != nil {
		rCtx.AppLogger.Fatal(err)
	}
	if err := rCtx.createClientServer(); err != nil {
		rCtx.AppLogger.Fatal(err)
	}

	tree, err := rCtx.compilePromise()
	if err != nil {
		rCtx.AppLogger.Fatal(err)
	}

	if err := rCtx.sendPromise(tree); err != nil {
		rCtx.AppLogger.Fatal(err)
	}
}
