package promise

import (
	"strings"

	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

type LogType int

const (
	LogTypeInfo LogType = iota
	LogTypeWarning
	LogTypeError
)

type LogPromise struct {
	Args []Argument
	Type LogType
}

func (p LogPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) != 0 {
		return nil, errors.New("a (info|error|warn) promise cannot have nested promises")
	}

	if len(args) < 1 {
		return nil, errors.New("a (info|error|warn) promise needs at least one format string argument")
	}

	return LogPromise{Type: p.Type, Args: args}, nil
}

func (p LogPromise) Desc(arguments []Constant) string {
	args := make([]string, len(p.Args))
	for i, v := range p.Args {
		args[i] = v.GetValue(arguments, &Variables{})
	}

	return "(info|error|warn " + strings.Join(args, " ") + ")"
}

func (p LogPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	fmtString := p.Args[0].GetValue(arguments, &Variables{})

	args := make([]interface{}, len(p.Args)-1)
	for i, v := range p.Args[1:] {
		args[i] = v.GetValue(arguments, &Variables{})
	}

	switch p.Type {
	case LogTypeInfo:
		logging.Logger.Infof(fmtString, args...)
	case LogTypeWarning:
		logging.Logger.Warnings++
		logging.Logger.Warnf(fmtString, args...)
	case LogTypeError:
		logging.Logger.Errors++
		logging.Logger.Errorf(fmtString, args...)
	}

	return true
}
