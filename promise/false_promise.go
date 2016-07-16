package promise

import "github.com/juju/errors"

type FalsePromise struct {
	Promise Promise
}

func (p FalsePromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) != 1 {
		return nil, errors.New("(false) can only have one nested promise")
	}

	if len(args) != 0 {
		return nil, errors.New("string args are not allowed in (false) promises")
	}

	return FalsePromise{children[0]}, nil
}

func (p FalsePromise) Desc(arguments []Constant) string {
	return "(false " + p.Promise.Desc(arguments) + ")"
}

func (p FalsePromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	p.Promise.Eval(arguments, ctx, stack)
	return false
}
