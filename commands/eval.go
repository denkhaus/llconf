package commands

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func Eval(ctx *cli.Context, logger *logrus.Logger) {
	rCtx, err := NewRunCtx(ctx, logger)
	if err != nil {
		rCtx.AppLogger.Error(err)
	}
	if _, err := rCtx.compilePromise(); err != nil {
		rCtx.AppLogger.Fatal(err)
	}

	rCtx.AppLogger.Info("evaluation successful")
}
