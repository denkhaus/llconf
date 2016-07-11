package promise

import "github.com/juju/errors"

type OrPromise struct {
	Promises []Promise
}

func (p OrPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) < 2 {
		return nil, errors.New("(or) needs at least 2 nested promises")
	}

	if len(args) != 0 {
		return nil, errors.New("string args are not allowed in (or) promises")
	}

	return OrPromise{children}, nil
}

func (p OrPromise) Desc(arguments []Constant) string {
	promises := ""
	for _, v := range p.Promises {
		promises += " " + v.Desc(arguments)
	}
	return "(or" + promises + ")"
}

func (p OrPromise) Eval(arguments []Constant, ctx *Context, stack string) error {
	for _, v := range p.Promises {
		if err := v.Eval(arguments, ctx, stack); err == nil {
			return err
		}
	}
	return errors.New("or not fulfilled")
}
