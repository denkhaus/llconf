package promise

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/util"
	"github.com/juju/errors"
)

type RestartPromise struct {
	NewExe Argument
}

func (p RestartPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(args) != 1 {
		return nil, errors.New("(restart) needs exactly 1 argument")
	}

	if len(children) != 0 {
		return nil, errors.New("(restart) cannot have nested promises")
	}

	return RestartPromise{args[0]}, nil
}

func (p RestartPromise) Desc(arguments []Constant) string {
	args := make([]string, len(arguments))
	for i, v := range arguments {
		args[i] = v.String()
	}
	return "(restart " + strings.Join(args, ", ") + ")"
}

func (p RestartPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	newExe := p.NewExe.GetValue(arguments, &ctx.Vars)
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

	logging.Logger.Infof("restarting llconf: llconf %v", ctx.Args)
	if _, err := p.restartLLConf(oldExe, ctx.Args, ctx.ExecOutput, ctx.ExecOutput); err != nil {
		logging.Logger.Error(errors.Annotate(err, "restart llconf"))
		return false
	}

	os.Exit(0)
	return true
}

func (p RestartPromise) restartLLConf(exe string, args []string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmd := exec.Command(exe, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	err := cmd.Start()
	return cmd, err
}
