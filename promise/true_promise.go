package promise

import "github.com/juju/errors"

type TruePromise struct {
	Promise Promise
}

func (p TruePromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) != 1 {
		return nil, errors.New("(true) can only have one nested promise")
	}

	if len(args) != 0 {
		return nil, errors.New("string args are not allowed in (true) promises")
	}

	return TruePromise{children[0]}, nil
}

func (p TruePromise) Desc(arguments []Constant) string {
	return "(true " + p.Promise.Desc(arguments) + ")"
}

func (p TruePromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	p.Promise.Eval(arguments, ctx, stack)
	return true
}
