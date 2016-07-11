package promise

import "fmt"

type InDir struct {
	Dir     Argument
	Promise Promise
}

func (p InDir) Desc(arguments []Constant) string {
	return fmt.Sprintf("(indir %s %s)", p.Dir, p.Promise.Desc(arguments))
}

func (p InDir) Eval(arguments []Constant, ctx *Context, stack string) error {
	copyied_ctx := *ctx
	copyied_ctx.InDir = p.Dir.GetValue(arguments, &ctx.Vars)
	return p.Promise.Eval(arguments, &copyied_ctx, stack)
}

func (p InDir) New(children []Promise, args []Argument) (Promise, error) {

	if len(args) != 1 {
		return nil, fmt.Errorf("(indir) needs exactly on argument, found %d", len(args))
	}

	if len(children) != 1 {
		return nil, fmt.Errorf("(indir) needs exactly on child promise, found %d", len(children))
	}

	return InDir{args[0], children[0]}, nil
}

//func (p InDir) Marshal(writer io.Writer) error {
//	if err := p.Dir.Marshal(writer); err != nil {
//		return err
//	}
//	if err := p.Promise.Marshal(writer); err != nil {
//		return err
//	}

//	return nil
//}
