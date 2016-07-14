package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

func NewServerCommand() cli.Command {
	return cli.Command{
		Name: "server",
		Subcommands: []cli.Command{
			{
				Name: "run",
				Action: func(ctx *cli.Context) error {
					rCtx, err := context.New(ctx, false)
					if err != nil {
						return errors.Annotate(err, "new run context")
					}
					defer rCtx.Close()

					if err := rCtx.CreateServer(); err != nil {
						return errors.Annotate(err, "create server")
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
								Usage: "the client-id the cert belongs to",
							},
							cli.StringFlag{
								Name:  "path",
								Usage: "path to the cert file",
							},
						},
						Action: func(ctx *cli.Context) error {
							rCtx, err := context.New(ctx, false)
							if err != nil {
								return errors.Annotate(err, "new run context")
							}
							defer rCtx.Close()

							clientID := ctx.String("id")
							path := ctx.String("path")
							if err := rCtx.AddCert(clientID, path); err != nil {
								return errors.Annotate(err, "register client cert")
							}

							logging.Logger.Infof("client certificate for id %q successfull saved", clientID)
							return nil
						},
					},
					{
						Name: "rm",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "id",
								Usage: "the client-id the cert belongs to",
							},
						},
						Action: func(ctx *cli.Context) error {
							rCtx, err := context.New(ctx, false)
							if err != nil {
								return errors.Annotate(err, "new run context")
							}
							defer rCtx.Close()

							clientID := ctx.String("id")
							if err := rCtx.RemoveCert(clientID); err != nil {
								return errors.Annotate(err, "remove client cert")
							}

							logging.Logger.Infof("client certificate for id %q successfull removed", clientID)
							return nil
						},
					},
				},
			},
		},
	}
}
