package cmd

import (
	"time"

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

					loop := make(chan int)

					for {
						go func(q chan int) {
							time.Sleep(time.Duration(rCtx.Interval) * time.Second)
							q <- 0
						}(loop)

						tree, err := rCtx.CompilePromise()
						if err != nil {
							return errors.Annotate(err, "compile promise")
						}

						if err := rCtx.SendPromise(tree); err != nil {
							return errors.Annotate(err, "send promise")
						}

						<-loop
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
