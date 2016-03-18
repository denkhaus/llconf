package commands

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func Run(ctx *cli.Context, logger *logrus.Logger) {
	rCtx := NewRunCtx(ctx, logger)
	tree, err := rCtx.compilePromise()
	if err != nil {
		rCtx.AppLogger.Error(err)
		return
	}

	rCtx.execPromise(tree)
}
