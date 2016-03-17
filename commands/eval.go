package commands

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/compiler"
	"github.com/sirupsen/logrus"
)

func Eval(ctx *cli.Context, logger *logrus.Logger) {
	args := ctx.Args()
	promise := ctx.GlobalString("promise")
	if promise == "" {
		logger.Fatal("config: no root promise specified")
	}

	var input string
	switch len(args) {
	case 0:
		logger.Fatal("no input folder specified")
	case 1:
		input = args.First()
	default:
		logger.Fatal("argument count mismatch")
	}

	promises, err := compiler.Compile(input)
	if err != nil {
		logger.Errorf("error while parsing input: %v", err)
		return
	}

	if _, ok := promises[promise]; !ok {
		logger.Errorf("specified goal (%s) not found in config", promise)
		return
	}

	logger.Info("evaluation successful")
}
