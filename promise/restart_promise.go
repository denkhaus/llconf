package promise

import (
	"os"
	"strings"
	"syscall"

	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/util"
	"github.com/juju/errors"
)

type RestartPromise struct {
	Args []Argument
}

func (p RestartPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(args) > 1 {
		return nil, errors.New("(restart) allows max 1 argument")
	}

	if len(children) != 0 {
		return nil, errors.New("(restart) cannot have nested promises")
	}

	return RestartPromise{args}, nil
}

func (p RestartPromise) Desc(arguments []Constant) string {
	args := make([]string, len(arguments))
	for i, v := range arguments {
		args[i] = v.String()
	}
	return "(restart " + strings.Join(args, ", ") + ")"
}

func (p RestartPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	var newExe string
	if len(p.Args) == 1 {
		newExe = p.Args[0].GetValue(arguments, &ctx.Vars)
	}

	if newExe != "" {
		if !util.FileExists(newExe) {
			panic(errors.Errorf("(restart) new executable %q is not present", newExe))
		}

		oldExe, ok := ctx.Vars["executable"]
		if !ok {
			panic(errors.Errorf("(restart) context var \"executable\" is not defined"))
		}

		if oldExe != newExe {
			if err := os.Rename(newExe, oldExe); err != nil {
				panic(errors.Annotatef(err, "(restart) mv %q to %q", newExe, oldExe))
			}
		}
	}

	ownPid := os.Getpid()

	logging.Logger.Infof("restarting llconf : llconf %v", ctx.Args)
	logging.Logger.Infof("sending signal %q to process %d", syscall.SIGUSR2, ownPid)
	// send ourselves a syscall.SIGUSR2 signal to restart
	syscall.Kill(ownPid, syscall.SIGUSR2)
	return true
}
