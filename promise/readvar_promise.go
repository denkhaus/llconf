package promise

import (
	"errors"
	"io"
	"strings"
)

type ReadvarPromise struct {
	VarName Argument
	Exec    Promise
}

type ReadvarWriter struct {
	writer io.Writer
	bytes  []byte
}

func (w *ReadvarWriter) Write(p []byte) (n int, err error) {
	w.bytes = append(w.bytes, p...)
	return w.writer.Write(p)
}

func (p ReadvarPromise) New(children []Promise, args []Argument) (Promise, error) {
	promise := ReadvarPromise{}

	if len(args) == 1 {
		promise.VarName = args[0]
	} else {
		return nil, errors.New("(readvar) needs exactly one variable name")
	}

	if len(children) != 1 {
		return nil, errors.New("(readvar) needs exactly one exec promise allowed")
	}

	exec := children[0]
	switch exec.(type) {
	case ExecPromise:
	case PipePromise:
	case NamedPromise:
		promise.Exec = exec
	default:
		return nil, errors.New("(readvar) did not found an evaluable promise")
	}

	return promise, nil
}

func (p ReadvarPromise) Desc(arguments []Constant) string {
	args := make([]string, len(arguments))
	for i, v := range arguments {
		args[i] = v.String()
	}
	return "(readvar " + strings.Join(args, ", ") + ")"
}

func (p ReadvarPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	result := p.Exec.Eval(arguments, ctx, stack)

	name := p.VarName.GetValue(arguments, &ctx.Vars)
	value := ctx.ExecOutput.String()

	ctx.Vars[name] = strings.TrimSpace(value)

	return result
}

//func (p ReadvarPromise) Marshal(writer io.Writer) error {
//	if err := p.VarName.Marshal(writer); err != nil {
//		return err
//	}
//	if err := p.Exec.Marshal(writer); err != nil {
//		return err
//	}

//	return nil
//}
