package promise

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
)

type SetvarPromise struct {
	Name  Argument
	Value Argument
}

func (p SetvarPromise) New(children []Promise, args []Argument) (Promise, error) {
	setvar := SetvarPromise{}

	if len(args) == 2 {
		setvar.Name = args[0]
		setvar.Value = args[1]
		return setvar, nil
	}

	return nil, errors.New("use (setvar \"varname\" \"varvalue\")")
}

func (p SetvarPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	name := p.Name.GetValue(arguments, &ctx.Vars)
	value := p.Value.GetValue(arguments, &ctx.Vars)

	if _, ok := ctx.Vars[name]; ok {
		panic(errors.Errorf("variable %q is already defined", name))
	}

	ctx.Vars[name] = strings.TrimSpace(value)
	return true
}

func (p SetvarPromise) Desc(arguments []Constant) string {
	return fmt.Sprintf("(setvar %q %q)", p.Name.String(), p.Value.String())
}
