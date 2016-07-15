package promise

import "github.com/juju/errors"

type NotPromise struct {
	Promise Promise
}

func (p NotPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) != 1 {
		return nil, errors.New("(not) can only have one nested promise")
	}

	if len(args) != 0 {
		return nil, errors.New("string args are not allowed in (not) promises")
	}

	return NotPromise{children[0]}, nil
}

func (p NotPromise) Desc(arguments []Constant) string {
	return "(not " + p.Promise.Desc(arguments) + ")"
}

func (p NotPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	if p.Promise.Eval(arguments, ctx, stack) {
		return false
	}

	return true
}
