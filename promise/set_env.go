package promise

import "fmt"

type SetEnv struct {
	Name  Argument
	Value Argument
	Child Promise
}

func (s SetEnv) Desc(arguments []Constant) string {
	return fmt.Sprintf("(setenv %s %s)", s.Name, s.Child.Desc(arguments))
}

func (s SetEnv) Eval(arguments []Constant, ctx *Context, stack string) bool {
	name := s.Name.GetValue(arguments, &ctx.Vars)
	value := s.Value.GetValue(arguments, &ctx.Vars)

	copyied_ctx := *ctx
	copyied_ctx.Env = append(copyied_ctx.Env, fmt.Sprintf("%s=%s", name, value))
	return s.Child.Eval(arguments, &copyied_ctx, stack)
}

func (s SetEnv) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) != 1 {
		return nil, fmt.Errorf("(setenv) needs one promise, found %d", len(args))
	}

	if len(args) != 2 {
		return nil, fmt.Errorf("(setenv) needs two arguments, found %d", len(args))
	}

	return SetEnv{args[0], args[1], children[0]}, nil
}

//func (p SetEnv) Marshal(writer io.Writer) error {
//	if err := p.Name.Marshal(writer); err != nil {
//		return err
//	}
//	if err := p.Value.Marshal(writer); err != nil {
//		return err
//	}
//	if err := p.Child.Marshal(writer); err != nil {
//		return err
//	}
//	return nil
//}
