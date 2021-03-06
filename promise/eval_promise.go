package promise

import (
	"fmt"

	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/util"
	"github.com/juju/errors"
)

//////////////////////////////////////////////////////////////////////////////////
type EvalPromise struct {
	RootPromise Argument
	InputPath   Argument
}

//////////////////////////////////////////////////////////////////////////////////
func (p EvalPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) > 0 {
		return nil, errors.New("(eval) allowes no nested promises")
	}

	if len(args) != 2 {
		return nil, errors.New("(eval) needs 2 parameters")
	}

	ep := EvalPromise{
		RootPromise: args[0],
		InputPath:   args[1],
	}
	return ep, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p EvalPromise) compilePromise(ctx *Context, inputPath, rootPromise string) (Promise, error) {
	logging.Logger.Info("compile eval promise")

	libDir, ok := ctx.Vars["lib_dir"]
	if !ok {
		return nil, errors.New("(eval) library dir is not defined")
	}

	if !util.FileExists(libDir) {
		return nil, errors.Errorf("(eval) library dir %q is not present", libDir)
	}

	promises, err := ctx.Compile(libDir, inputPath)
	if err != nil {
		return nil, errors.Annotate(err, "compile promise")
	}

	tree, ok := promises[rootPromise]
	if !ok {
		return nil, errors.New("root promise (" + rootPromise + ") unknown")
	}

	return tree, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p EvalPromise) Desc(arguments []Constant) string {
	return fmt.Sprintf("(eval %q %q )", p.RootPromise.String(), p.InputPath.String())
}

//////////////////////////////////////////////////////////////////////////////////
func (p EvalPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	rootPromise := p.RootPromise.GetValue(arguments, &ctx.Vars)
	inputPath := p.InputPath.GetValue(arguments, &ctx.Vars)

	if rootPromise == "" {
		panic(errors.Errorf("(eval) root promise %q is undefined", rootPromise))
	}

	if !util.FileExists(inputPath) {
		panic(errors.Errorf("(eval) input path %q does not exist", inputPath))
	}

	promise, err := p.compilePromise(ctx, inputPath, rootPromise)
	if err != nil {
		panic(errors.Annotatef(err, "(eval) compile promise"))
	}

	copyied_ctx := *ctx
	copyied_ctx.InDir = inputPath
	res := promise.Eval([]Constant{}, &copyied_ctx, "")

	return res
}
