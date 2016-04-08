package commands

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	libpromise "github.com/denkhaus/llconf/promise"
)

func Serve(ctx *cli.Context, logger *logrus.Logger) {
	rCtx, err := NewRunCtx(ctx, logger)
	if err != nil {
		rCtx.AppLogger.Fatal(err)
	}
	if err := rCtx.setupLogging(); err != nil {
		rCtx.AppLogger.Fatal(err)
	}

	quit := make(chan int)
	var tree libpromise.Promise

	for {
		go func(q chan int) {
			time.Sleep(time.Duration(rCtx.Interval) * time.Second)
			q <- 0
		}(quit)

		if npt, err := rCtx.compilePromise(); err != nil {
			rCtx.AppLogger.Error(err)
		} else {
			tree = npt
		}

		if tree != nil {
			rCtx.execPromise(tree)
		} else {
			rCtx.AppLogger.Error("could not find any valid promises")
		}

		<-quit
	}
}
