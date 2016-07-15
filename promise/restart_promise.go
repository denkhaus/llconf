package promise

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/denkhaus/llconf/logging"
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
	newexe := p.NewExe.GetValue(arguments, &ctx.Vars)
	if _, err := os.Stat(newexe); err != nil {
		logging.Logger.Error(errors.Annotate(err, "stat"))
		return false
	}

	exe := filepath.Clean(os.Args[0])

	os.Rename(newexe, exe)
	logging.Logger.Infof("restarted llconf: llconf %v", ctx.Args)
	if _, err := p.restartLLConf(exe, ctx.Args, ctx.ExecOutput, ctx.ExecOutput); err != nil {
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
