package commands

import (
	"errors"
	"flag"
	"fmt"
	"log/syslog"
	"os"
	"path/filepath"
	"time"

	syslogger "github.com/Sirupsen/logrus/hooks/syslog"

	"bytes"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/compiler"
	libpromise "github.com/denkhaus/llconf/promise"
)

var serve_cfg struct {
	root_promise string
	verbose      bool
	use_syslog   bool
	interval     int
	inp_dir      string
	workdir      string
	runlog_path  string
}

func Serve(ctx *cli.Context, logger *logrus.Logger) {
	parseArguments(ctx, logger)
	logger, err := setupLogging(logger)
	if err != nil {
		logger.Fatal(err)
	}

	quit := make(chan int)
	var promise_tree libpromise.Promise

	for {
		go func(q chan int) {
			time.Sleep(time.Duration(serve_cfg.interval) * time.Second)
			q <- 0
		}(quit)

		if npt, err := updatePromise(serve_cfg.inp_dir, serve_cfg.root_promise); err != nil {
			logger.Errorf("error while parsing input folder: %v", err)
		} else {
			promise_tree = npt
		}

		if promise_tree != nil {
			checkPromise(promise_tree, logger, flag.Args())
		} else {
			logger.Error("could not find any valid promises")
		}

		<-quit
	}
}

func setupLogging(logger *logrus.Logger) (*logrus.Logger, error) {
	if serve_cfg.use_syslog {
		hook, err := syslogger.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err != nil {
			return logger, err
		}

		logger.Hooks.Add(hook)
	}

	return logger, nil
}

func parseArguments(ctx *cli.Context, logger *logrus.Logger) {
	args := ctx.Args()

	switch len(args) {
	case 0:
		logger.Fatal("config: no workdir specified")
	case 1:
		serve_cfg.workdir = args.First()
	default:
		logger.Fatal("config: argument count mismatch")
	}

	serve_cfg.inp_dir = ctx.String("input-folder")
	if serve_cfg.inp_dir == "" {
		serve_cfg.inp_dir = filepath.Join(serve_cfg.workdir, "input")
	}
	serve_cfg.runlog_path = ctx.String("runlog")
	if serve_cfg.runlog_path == "" {
		serve_cfg.runlog_path = filepath.Join(serve_cfg.workdir, "runlog")
	}

	serve_cfg.root_promise = ctx.GlobalString("promise")
	serve_cfg.interval = ctx.Int("interval")
	serve_cfg.use_syslog = ctx.Bool("syslog")
	serve_cfg.verbose = ctx.Bool("verbose")

	// when run as daemon, the home folder isn't set
	if os.Getenv("HOME") == "" {
		os.Setenv("HOME", serve_cfg.workdir)
	}
}

func updatePromise(folder, root string) (libpromise.Promise, error) {
	promises, err := compiler.Compile(folder)
	if err != nil {
		return nil, err
	}

	if promise, ok := promises[root]; !ok {
		return nil, errors.New("root promise (" + root + ") unknown")
	} else {
		return promise, nil
	}
}

func checkPromise(p libpromise.Promise, logger *logrus.Logger, args []string) {
	vars := libpromise.Variables{}
	vars["input_dir"] = serve_cfg.inp_dir
	vars["work_dir"] = serve_cfg.workdir
	vars["executable"] = filepath.Clean(os.Args[0])

	log := libpromise.Logger{}
	log.Logger = logger

	ctx := libpromise.Context{
		Logger:     &log,
		ExecOutput: &bytes.Buffer{},
		Vars:       vars,
		Args:       args,
		Env:        []string{},
		Verbose:    serve_cfg.verbose,
		InDir:      "",
	}

	starttime := time.Now().Local()
	fullfilled := p.Eval([]libpromise.Constant{}, &ctx, "")
	endtime := time.Now().Local()

	logger.Infof("%d changes and %d tests executed", ctx.Logger.Changes, ctx.Logger.Tests)
	if fullfilled {
		logger.Infof("evaluation successful")
	} else {
		logger.Error("error during evaluation")
	}

	writeRunLog(fullfilled, ctx.Logger.Changes,
		ctx.Logger.Tests, starttime, endtime, serve_cfg.runlog_path)
}

func writeRunLog(success bool, changes, tests int,
	starttime, endtime time.Time, path string) (err error) {
	var output string

	duration := endtime.Sub(starttime)

	if success {
		output = fmt.Sprintf("successful, endtime=%d, duration=%f, c=%d, t=%d\n",
			endtime.Unix(), duration.Seconds(), changes, tests)
	} else {
		output = fmt.Sprintf("error, endtime=%d, duration=%f, c=%d, t=%d\n",
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
