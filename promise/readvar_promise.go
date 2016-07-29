package promise

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
)

type ReadvarPromise struct {
	VarName Argument
	Exec    Promise
}

func (p ReadvarPromise) New(children []Promise, args []Argument) (Promise, error) {
	promise := ReadvarPromise{}

	if len(args) == 1 {
		promise.VarName = args[0]
	} else {
		return nil, errors.New("(readvar) needs exactly one variable name")
	}

	if len(children) != 1 {
		return nil, errors.New("(readvar) needs exactly one exec promise")
	}

	exec := children[0]

	switch exec.(type) {
	case ExecPromise, PipePromise, NamedPromise:
		promise.Exec = exec
	default:
		return nil, errors.New("(readvar) did not found an evaluable promise")
	}

	return promise, nil
}

func (p ReadvarPromise) Desc(arguments []Constant) string {
	return fmt.Sprintf("(readvar %s %s)", p.VarName, p.Exec.Desc(arguments))
}

func (p ReadvarPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	result := p.Exec.Eval(arguments, ctx, stack)
	name := p.VarName.GetValue(arguments, &ctx.Vars)
	value := ctx.ExecOutput.String()

	val := strings.TrimSpace(value)
	if v, ok := ctx.Vars[name]; ok && v != val {
		panic(errors.Errorf("variable %q is already defined", name))
	}

	ctx.Vars[name] = val
	return result
}
