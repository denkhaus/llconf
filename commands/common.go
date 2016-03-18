package commands

import (
	"bytes"
	"errors"
	"fmt"
	"log/syslog"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	syslogger "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/compiler"
	libpromise "github.com/denkhaus/llconf/promise"
)

type RunCtx struct {
	RootPromise string
	Verbose     bool
	UseSyslog   bool
	Interval    int
	InputDir    string
	WorkDir     string
	RunlogPath  string
	AppCtx      *cli.Context
	AppLogger   *logrus.Logger
}

func NewRunCtx(ctx *cli.Context, logger *logrus.Logger) *RunCtx {
	rCtx := RunCtx{AppCtx: ctx, AppLogger: logger}
	rCtx.parseArguments()
	return &rCtx
}

func (p *RunCtx) setupLogging() error {
	if p.UseSyslog {
		hook, err := syslogger.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err != nil {
			return err
		}

		p.AppLogger.Hooks.Add(hook)
	}

	return nil
}

func (p *RunCtx) compilePromise() (libpromise.Promise, error) {
	promises, err := compiler.Compile(p.InputDir)
	if err != nil {
		return nil, fmt.Errorf("parsing input folder: %v", err)
	}

	tree, ok := promises[p.RootPromise]
	if !ok {
		return nil, errors.New("root promise (" + p.RootPromise + ") unknown")
	}

	return tree, nil
}

func (p *RunCtx) parseArguments() {
	args := p.AppCtx.Args()

	switch len(args) {
	case 0:
		p.AppLogger.Fatal("config: no workdir specified")
	case 1:
		p.WorkDir = args.First()
	default:
		p.AppLogger.Fatal("config: argument count mismatch")
	}

	p.InputDir = p.AppCtx.String("input-folder")
	if p.InputDir == "" {
		p.InputDir = filepath.Join(p.WorkDir, "input")
	}
	p.RunlogPath = p.AppCtx.String("runlog")
	if p.RunlogPath == "" {
		p.RunlogPath = filepath.Join(p.WorkDir, "runlog")
	}

	p.RootPromise = p.AppCtx.GlobalString("promise")
	p.Interval = p.AppCtx.Int("interval")
	p.UseSyslog = p.AppCtx.Bool("syslog")
	p.Verbose = p.AppCtx.Bool("verbose")

	// when run as daemon, the home folder isn't set
	if os.Getenv("HOME") == "" {
		os.Setenv("HOME", p.WorkDir)
	}
}

func (p *RunCtx) execPromise(tree libpromise.Promise) {
	vars := libpromise.Variables{}
	vars["input_dir"] = p.InputDir
	vars["work_dir"] = p.WorkDir
	vars["executable"] = filepath.Clean(os.Args[0])

	log := libpromise.Logger{}
	log.Logger = p.AppLogger

	ctx := libpromise.Context{
		Logger:     &log,
		ExecOutput: &bytes.Buffer{},
		Vars:       vars,
		Args:       p.AppCtx.Args(),
		Env:        []string{},
		Verbose:    p.Verbose,
		InDir:      "",
	}

	starttime := time.Now().Local()
	fullfilled := tree.Eval([]libpromise.Constant{}, &ctx, "")
	endtime := time.Now().Local()

	p.AppLogger.Infof("%d changes and %d tests executed", ctx.Logger.Changes, ctx.Logger.Tests)
	if fullfilled {
		ctx.Logger.Infof("evaluation successful")
	} else {
		ctx.Logger.Error("error during evaluation")
	}

	writeRunLog(fullfilled, ctx.Logger.Changes,
		ctx.Logger.Tests, starttime, endtime, p.RunlogPath)
}

func writeRunLog(success bool, changes, tests int,
	starttime, endtime time.Time, path string) (err error) {
	var output string

	duration := endtime.Sub(starttime)

	if success {
		output = fmt.Sprintf("successful, endtime=%d, duration=%f, c=%d, t=%d",
			endtime.Unix(), duration.Seconds(), changes, tests)
	} else {
		output = fmt.Sprintf("error, endtime=%d, duration=%f, c=%d, t=%d",
			endtime.Unix(), duration.Seconds(), changes, tests)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	data := []byte(output)
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		return
	}

	err = f.Close()
	return
}
