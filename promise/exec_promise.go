package promise

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	wgOutput  sync.WaitGroup
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

	if err := ctx.sanitizeInDir(); err != nil {
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

////////////////////////////////////////////////////////////////////////////////
func (p ExecPromise) processOutput(ctx *Context, cmd *exec.Cmd) error {
	ctx.ExecOutput.Reset()
	cw := bufio.NewWriter(ctx.ExecOutput)

	process := func(reader io.Reader, fn func(string)) {
		p.wgOutput.Add(1)
		defer func() {
			cw.Flush()
			p.wgOutput.Done()

		}()

		scn := bufio.NewScanner(reader)
		for scn.Scan() {
			cw.WriteString(scn.Text())
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
		panic(errors.Annotate(err, "get command"))
	}

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

	if err := p.processOutput(ctx, cmd); err != nil {
		panic(errors.Annotate(err, "process output"))
	}
	if err := cmd.Start(); err != nil {
		panic(errors.Annotate(err, "cmd start"))
	}

	var ret = true
	if err := cmd.Wait(); err != nil {
		ret = false
	}

	//wait until output is processed
	p.wgOutput.Wait()

	if ctx.Verbose || p.Type == ExecChange {
		logging.Logger.Info(stack)
		logging.Logger.Infof("[%s %s]%s -> %t", p.Type.String(),
			strings.Join(cmd.Args, " "),
			ctx.ExecOutput.String(), ret)
	}

	p.Type.IncrementExecCounter()
	return ret
}

////////////////////////////////////////////////////////////////////////////////

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
			panic(errors.Annotate(err, "get command"))
		}

		v.Type.IncrementExecCounter()
		cstrings = append(cstrings, "["+v.Type.String()+"] "+strings.Join(cmd.Args, " "))
		commands = append(commands, cmd)

		if v.Type == ExecChange {
			pipe_contains_change = true
		}
	}

	for i, command := range commands[:len(commands)-1] {
		out, err := command.StdoutPipe()
		if err != nil {
			panic(errors.Annotate(err, "stdout pipe"))
		}

		if err := command.Start(); err != nil {
			panic(errors.Annotate(err, "start"))
		}

		commands[i+1].Stdin = out
	}

	last_cmd := commands[len(commands)-1]

	ctx.ExecOutput.Reset()
	last_cmd.Stdout = ctx.ExecOutput
	last_cmd.Stderr = ctx.ExecOutput

	cmdError := last_cmd.Run()
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
	return (cmdError == nil)
}
