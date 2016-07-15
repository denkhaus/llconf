package promise

import "github.com/juju/errors"

type AndPromise struct {
	Promises []Promise
}

func (p AndPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) < 2 {
		return nil, errors.New("(and) needs at least 2 nested promises")
	}

	if len(args) != 0 {
		return nil, errors.New("string args are not allowed in (and) promises")
	}

	return AndPromise{children}, nil
}

func (p AndPromise) Desc(arguments []Constant) string {
	promises := ""
	for _, v := range p.Promises {
		promises += " " + v.Desc(arguments)
	}
	return "(and" + promises + ")"
}

func (p AndPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	for _, v := range p.Promises {
		if !v.Eval(arguments, ctx, stack) {
			return false
		}
	}
	return true
}
