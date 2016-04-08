package promise

import "fmt"

type OrPromise struct {
	Promises []Promise
}

func (p OrPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) == 0 {
		return nil, fmt.Errorf("(and) needs at least 1 nested promise")
	}

	if len(args) != 0 {
		return nil, fmt.Errorf("string args are not allowed in (and) promises")
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

func (p OrPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	for _, v := range p.Promises {
		r := v.Eval(arguments, ctx, stack)
		if r == true {
			return true
		}
	}
	return false
}

//func (p OrPromise) Marshal(writer io.Writer) error {
//	for _, pr := range p.Promises {
//		if err := pr.Marshal(writer); err != nil {
//			return err
//		}
//	}

//	return nil
//}
