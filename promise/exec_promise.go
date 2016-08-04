package promise

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/util"
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
	c := p.Arguments[0].GetValue(arguments, &ctx.Vars)
	largs := p.Arguments[1:]

	args := []string{}
	for _, argument := range largs {
		args = append(args, argument.GetValue(arguments, &ctx.Vars))
	}

	cmd := exec.Command(c, args...)

	if ctx.InDir != "" {
		// use (in_dir) for command lookup
		if newcmd, err := exec.LookPath(filepath.Join(ctx.InDir, c)); err == nil {
			cmd = exec.Command(newcmd, args...)
		}

		cmd.Dir = ctx.InDir
	} else {
		cmd.Dir = os.Getenv("PWD")
	}

	if ctx.Credential != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: ctx.Credential,
		}
	}

	cmd.Env = os.Environ()
	for _, v := range ctx.Env {
		cmd.Env = append(cmd.Env, v)
	}

	return cmd, nil
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
	ctx.ExecStdout.Reset()
	ctx.ExecStderr.Reset()

	process := func(reader io.Reader, writer io.Writer) {
		cw := bufio.NewWriter(writer)
		p.wgOutput.Add(1)
		defer func() {
			cw.Flush()
			p.wgOutput.Done()
		}()

		scn := bufio.NewScanner(reader)
		for scn.Scan() {
			cw.WriteString(scn.Text())
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

	go process(outReader, ctx.ExecStdout)
	go process(errReader, ctx.ExecStderr)

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
		logging.Logger.Infof("[%s %s]-> %t", p.Type.String(), strings.Join(cmd.Args, " "), ret)
		processCmdOutput(ctx)
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

		cstrings = append(cstrings, "["+v.Type.String()+"] "+strings.Join(cmd.Args, " "))
		commands = append(commands, cmd)

		if v.Type == ExecChange {
			pipe_contains_change = true
		}
	}

	nCommands := len(commands)
	for i, command := range commands[:nCommands-1] {
		out, err := command.StdoutPipe()
		if err != nil {
			panic(errors.Annotate(err, "stdout pipe"))
		}

		if err := command.Start(); err != nil {
			panic(errors.Annotate(err, "start"))
		}

		p.Execs[i].Type.IncrementExecCounter()
		commands[i+1].Stdin = out
	}

	last_cmd := commands[len(commands)-1]

	ctx.ExecStdout.Reset()
	last_cmd.Stdout = ctx.ExecStdout
	last_cmd.Stderr = ctx.ExecStderr
	cmdError := last_cmd.Run()

	for _, command := range commands[:nCommands-1] {
		command.Wait()
	}

	if ctx.Verbose || pipe_contains_change {
		logging.Logger.Info(stack)
		logging.Logger.Info(strings.Join(cstrings, " | "))
		processCmdOutput(ctx)
	}
	return (cmdError == nil)
}

////////////////////////////////////////////////////////////////////////////////

type SPipePromise struct {
	Execs []ExecPromise
}

func (p SPipePromise) New(children []Promise, args []Argument) (Promise, error) {

	if len(args) != 0 {
		return nil, errors.New("string arguments not allowed in (spipe) promise")
	}

	execs := []ExecPromise{}

	for _, c := range children {
		switch t := c.(type) {
		case ExecPromise:
			execs = append(execs, t)
		default:
			return nil, errors.New("only (test) or (change) promises allowed inside (spipe) promise")
		}
	}

	return PipePromise{execs}, nil
}

func (p SPipePromise) Desc(arguments []Constant) string {
	retval := "(spipe"
	for _, v := range p.Execs {
		retval += " " + v.Desc(arguments)
	}
	return retval + ")"
}

func (p SPipePromise) Eval(arguments []Constant, ctx *Context, stack string) bool {

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

	nCommands := len(commands)
	for i, command := range commands[:nCommands-1] {
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

	ctx.ExecStdout.Reset()
	last_cmd.Stdout = ctx.ExecStdout
	last_cmd.Stderr = ctx.ExecStderr
	cmdError := last_cmd.Run()

	for _, command := range commands[:nCommands-1] {
		command.Wait()
	}

	if ctx.Verbose || pipe_contains_change {
		logging.Logger.Info(stack)
		logging.Logger.Info(strings.Join(cstrings, " | "))
		processCmdOutput(ctx)
	}
	return (cmdError == nil)
}

////////////////////////////////////////////////////////////////////////////////
func processCmdOutput(ctx *Context) {
	process := func(prefix string, buf *bytes.Buffer, outFunc func(string, ...interface{})) {
		str := util.NewStriplines()
		str.Write(buf.Bytes())
		str.Close()

		if str.HasContent() {
			if str.Lines() > 1 {
				outFunc("%s:\n%s", prefix, str.String())
			} else {
				outFunc("%s: %s", prefix, str.String())
			}
		}
	}

	process("stdout", ctx.ExecStdout, logging.Logger.Infof)
	process("stderr", ctx.ExecStderr, func(fmt string, args ...interface{}) {
		logging.Logger.Errors++
		logging.Logger.Errorf(fmt, args...)
	})
}
