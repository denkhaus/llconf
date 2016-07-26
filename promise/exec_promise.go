package promise

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

type ExecType int

const (
	ExecChange ExecType = iota
	ExecTest
)

func (t ExecType) Name() string {
	switch t {
	case ExecChange:
		return "change"
	case ExecTest:
		return "test"
	default:
		return "unknown"
	}
}

func (t ExecType) IncrementExecCounter() {
	if t == ExecChange {
		logging.Logger.Changes++
	}

	if t == ExecTest {
		logging.Logger.Tests++
	}
}

func (t ExecType) String() string {
	return t.Name()
}

type ExecPromise struct {
	Type      ExecType
	Arguments []Argument
}

func (p ExecPromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(children) != 0 {
		return nil, errors.Errorf("nested promises not allowed in (%s)", p.Type.Name())
	}

	if len(args) == 0 {
		return nil, errors.Errorf("(%s) needs at least 1 string argument", p.Type.Name())
	}

	return ExecPromise{Type: p.Type, Arguments: args}, nil
}

func (p ExecPromise) getCommand(arguments []Constant, ctx *Context) (*exec.Cmd, error) {

	cmd := p.Arguments[0].GetValue(arguments, &ctx.Vars)
	largs := p.Arguments[1:]

	args := []string{}
	for _, argument := range largs {
		args = append(args, argument.GetValue(arguments, &ctx.Vars))
	}

	if err := sanitizeInDir(ctx); err != nil {
		return nil, errors.Annotate(err, "sanitize indir")
	}

	command := exec.Command(cmd, args...)

	if ctx.InDir != "" {
		// use (in_dir) for command lookup
		if newcmd, err := exec.LookPath(filepath.Join(ctx.InDir, cmd)); err == nil {
			command = exec.Command(newcmd, args...)
		}

		command.Dir = ctx.InDir
	} else {
		command.Dir = os.Getenv("PWD")
	}

	command.Env = os.Environ()
	for _, v := range ctx.Env {
		command.Env = append(command.Env, v)
	}

	return command, nil
}

func (p ExecPromise) Desc(arguments []Constant) string {
	if len(p.Arguments) == 0 {
		return "(" + p.Type.Name() + ")"
	}

	cmd := p.Arguments[0].GetValue(arguments, &Variables{})
	largs := p.Arguments[1:]

	args := make([]string, len(largs))
	for i, v := range largs {
		args[i] = v.GetValue(arguments, &Variables{})
	}

	return "(" + p.Type.Name() + " <" + cmd + " [" + strings.Join(args, ", ") + "] >)"
}

func (p ExecPromise) processOutput(ctx *Context, cmd *exec.Cmd) error {
	ctx.ExecOutput.Reset()
	commonWriter := bufio.NewWriter(ctx.ExecOutput)

	process := func(reader io.Reader, fn func(string)) {
		scn := bufio.NewScanner(reader)
		for scn.Scan() {
			commonWriter.WriteString(scn.Text())
			if ctx.Verbose || p.Type == ExecChange {
				fn(scn.Text())
			}
		}
	}

	outReader, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Annotate(err, "get stdout pipe")
	}
	errReader, err := cmd.StderrPipe()
	if err != nil {
		return errors.Annotate(err, "get stderr pipe")
	}

	go process(outReader, func(out string) { logging.Logger.Info(out) })
	go process(errReader, func(out string) { logging.Logger.Error(out) })
	return nil
}

func (p ExecPromise) Eval(arguments []Constant, ctx *Context, stack string) bool {
	cmd, err := p.getCommand(arguments, ctx)
	if err != nil {
		logging.Logger.Error(errors.Annotate(err, "get command"))
		return false
	}

	if ctx.Verbose || p.Type == ExecChange {
		logging.Logger.Info(stack)
		logging.Logger.Info("[", p.Type.String(), "] ", strings.Join(cmd.Args, " "))
	}

	quit := make(chan bool)
	go func(quit chan bool) {
		select {
		case <-quit:
			return
		case <-time.After(time.Duration(5) * time.Minute):
			logging.Logger.Warn(stack, " has been running for 5 minutes")
		}
	}(quit)

	defer func() { quit <- true }()

	if err := p.processOutput(ctx, cmd); err != nil {
		logging.Logger.Error(errors.Annotate(err, "process output"))
		return false
	}
	if err = cmd.Start(); err != nil {
		logging.Logger.Error(errors.Annotate(err, "cmd start"))
		return false
	}
	if err = cmd.Wait(); err != nil {
		logging.Logger.Error(errors.Annotate(err, "cmd wait"))
		return false
	}

	p.Type.IncrementExecCounter()
	return true
}

/////////////////////////////

type PipePromise struct {
	Execs []ExecPromise
}

func (p PipePromise) New(children []Promise, args []Argument) (Promise, error) {

	if len(args) != 0 {
		return nil, errors.New("string arguments not allowed in (pipe) promise")
	}

	execs := []ExecPromise{}

	for _, c := range children {
		switch t := c.(type) {
		case ExecPromise:
			execs = append(execs, t)
		default:
			return nil, errors.New("only (test) or (change) promises allowed inside (pipe) promise")
		}
	}

	return PipePromise{execs}, nil
}

func (p PipePromise) Desc(arguments []Constant) string {
	retval := "(pipe"
	for _, v := range p.Execs {
		retval += " " + v.Desc(arguments)
	}
	return retval + ")"
}

func (p PipePromise) Eval(arguments []Constant, ctx *Context, stack string) bool {

	quit := make(chan bool)
	defer func() { quit <- true }()

	go func(quit chan bool) {
		select {
		case <-quit:
			return
		case <-time.After(time.Duration(5) * time.Minute):
			logging.Logger.Warn(stack, " has been running for 5 minutes")
		}
	}(quit)

	commands := []*exec.Cmd{}
	cstrings := []string{}

	pipe_contains_change := false

	for _, v := range p.Execs {
		cmd, err := v.getCommand(arguments, ctx)
		if err != nil {
			logging.Logger.Error(errors.Annotate(err, "get command"))
			return false
		} else {
			v.Type.IncrementExecCounter()
		}
		cstrings = append(cstrings, "["+v.Type.String()+"] "+strings.Join(cmd.Args, " "))
		commands = append(commands, cmd)

		if v.Type == ExecChange {
			pipe_contains_change = true
		}
	}

	for i, command := range commands[:len(commands)-1] {
		out, err := command.StdoutPipe()
		if err != nil {
			logging.Logger.Error(errors.Annotate(err, "stdout pipe"))
			return false
		}
		command.Start()
		commands[i+1].Stdin = out
	}

	last_cmd := commands[len(commands)-1]

	ctx.ExecOutput.Reset()
	last_cmd.Stdout = ctx.ExecOutput
	last_cmd.Stderr = ctx.ExecOutput

	err := last_cmd.Run()
	for _, command := range commands[:len(commands)-1] {
		command.Wait()
	}

	if ctx.Verbose || pipe_contains_change {
		logging.Logger.Info(stack)
		logging.Logger.Info(strings.Join(cstrings, " | "))
		if ctx.ExecOutput.Len() > 0 {
			logging.Logger.Info(ctx.ExecOutput.String())
		}
	}
	return (err == nil)
}

func sanitizeInDir(ctx *Context) error {
	if ctx.InDir == "" {
		return nil
	}

	if ctx.InDir[:2] == "~/" {
		usr, err := user.Current()
		if err != nil {
			return errors.Annotate(err, "get current user")
		}
		ctx.InDir = filepath.Join(usr.HomeDir, ctx.InDir[2:])
	}

	abs, err := filepath.Abs(ctx.InDir)
	if err != nil {
		return errors.Annotate(err, "make indir path absolute")
	}
	ctx.InDir = abs

	fs, err := os.Stat(ctx.InDir)
	if err != nil {
		return errors.Errorf("(indir) error for path %q: %s", ctx.InDir, err)
	}

	if !fs.IsDir() {
		return errors.Errorf("(indir) not a directory: %q", ctx.InDir)
	}
	return nil
}
