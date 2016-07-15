package cmd

import (
	"time"

	"golang.org/x/tools/godoc/util"

	"github.com/howeyc/fsnotify"

	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

func NewClientCommand() cli.Command {
	return cli.Command{
		Name: "client",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "input-folder, i",
				Usage:  "the folder containing input files",
				EnvVar: "LLCONF_INPUT_FOLDER",
			},
			cli.StringFlag{
				Name:   "promise, p",
				Usage:  "the root promise name",
				EnvVar: "LLCONF_PROMISE",
				Value:  "done",
			},
		},
		Subcommands: []cli.Command{
			{
				Name: "run",
				Action: func(ctx *cli.Context) error {
					rCtx, err := context.New(ctx, true, true)
					if err != nil {
						return errors.Annotate(err, "new run context")
					}
					defer rCtx.Close()

					if err := rCtx.CreateClient(); err != nil {
						return errors.Annotate(err, "create client")
					}

					tree, err := rCtx.CompilePromise()
					if err != nil {
						return errors.Annotate(err, "compile promise")
					}

					if err := rCtx.SendPromise(tree); err != nil {
						return errors.Annotate(err, "send promise")
					}

					return nil
				},
			},
			{
				Name: "test",
				Action: func(ctx *cli.Context) error {
					rCtx, err := context.New(ctx, true, true)
					if err != nil {
						return errors.Annotate(err, "new run context")
					}
					defer rCtx.Close()

					if _, err := rCtx.CompilePromise(); err != nil {
						return errors.Annotate(err, "compile promise")
					}

					logging.Logger.Info("test successful")
					return nil
				},
			},
			{
				Name: "watch",
				Flags: []cli.Flag{
					cli.IntFlag{
						Name:   "interval, n",
						Usage:  "set the minium time between promise-tree evaluation",
						EnvVar: "LLCONF_INTERVAL",
						Value:  300,
					},
				},
				Action: func(ctx *cli.Context) error {
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
						thr := util.NewThrottle(0.5, 1*time.Second)
						for {
							select {
							case ev := <-watcher.Event:
								logging.Logger.Infof("file changed: %v", ev.Name)
								thr.Throttle()
								trigger <- 0
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

					return nil
				},
			},
			{
				Name: "cert",
				Subcommands: []cli.Command{
					{
						Name: "add",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "id",
								Usage: "the server-id the cert belongs to",
							},
							cli.StringFlag{
								Name:  "path",
								Usage: "path to the cert file",
							},
						},
						Action: func(ctx *cli.Context) error {
							rCtx, err := context.New(ctx, true, false)
							if err != nil {
								return errors.Annotate(err, "new run context")
							}
							defer rCtx.Close()

							serverID := ctx.String("id")
							path := ctx.String("path")
							if err := rCtx.AddCert(serverID, path); err != nil {
								return errors.Annotate(err, "add server cert")
							}

							logging.Logger.Infof("server certificate for id %q successfull saved", serverID)
							return nil
						},
					},
					{
						Name: "rm",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "id",
								Usage: "the server-id the cert belongs to",
							},
						},
						Action: func(ctx *cli.Context) error {
							rCtx, err := context.New(ctx, true, false)
							if err != nil {
								return errors.Annotate(err, "new run context")
							}
							defer rCtx.Close()

							serverID := ctx.String("id")
							if err := rCtx.RemoveCert(serverID); err != nil {
								return errors.Annotate(err, "remove server cert")
							}

							logging.Logger.Infof("server certificate for id %q successfull removed", serverID)
							return nil
						},
					},
				},
			},
		},
	}
}
