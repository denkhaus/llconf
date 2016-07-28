package cmd

import (
	"time"

	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/util"
	"github.com/howeyc/fsnotify"
	"github.com/juju/errors"
)

func newClientWatchCommand() cli.Command {
	return cli.Command{
		Name: "watch",
		Action: func(ctx *cli.Context) error {
			if err := clientWatch(ctx); err != nil {
				logging.Logger.Error(err)
			}
			return nil
		},
	}
}

func clientWatch(ctx *cli.Context) error {
	logging.Logger.Info("exec: client watch")

	rCtx, err := context.New(ctx, true, true)
	if err != nil {
		return errors.Annotate(err, "new run context")
	}
	defer rCtx.Close()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Annotate(err, "new watcher")
	}
	defer watcher.Close()

	if err := watcher.Watch(rCtx.InputDir); err != nil {
		return errors.Annotate(err, "watch")
	}

	trigger := make(chan int)
	errch := make(chan error)

	go func() {
		thr := util.NewThrottle(0.8, 5*time.Second)
		for {
			select {
			case ev := <-watcher.Event:
				if !thr.Triggered() {
					logging.Logger.Infof("input file changed: %v", ev.Name)
					thr.Throttle()
					trigger <- 0
				}
			case err := <-watcher.Error:
				errch <- err
			}
		}
	}()

	for {

		if tree, err := rCtx.CompilePromise(); err != nil {
			logging.Logger.Error(errors.Annotate(err, "compile promise"))
		} else {
			if err := rCtx.CreateClient(); err != nil {
				return errors.Annotate(err, "create client")
			}
			if err := rCtx.SendPromise(tree); err != nil {
				return errors.Annotate(err, "send promise")
			}
		}

		select {
		case <-trigger:
			continue
		case err := <-watcher.Error:
			return errors.Annotate(err, "watcher error")
		}
	}
}
