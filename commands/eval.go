package commands

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func Eval(ctx *cli.Context, logger *logrus.Logger) {
	rCtx := NewRunCtx(ctx, logger)
	if _, err := rCtx.compilePromise(); err != nil {
		rCtx.AppLogger.Error(err)
		return
	}

	rCtx.AppLogger.Info("evaluation successful")
}
