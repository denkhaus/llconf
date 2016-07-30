package cmd

import (
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/context"
	"github.com/denkhaus/llconf/logging"
	"github.com/juju/errors"
)

func newClientCertCommand() cli.Command {
	return cli.Command{
		Name: "cert",
		Subcommands: []cli.Command{
			{
				Name: "add",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "id",
						Usage: "the id the cert belongs to",
					},
					cli.StringFlag{
						Name:  "path",
						Usage: "path to the cert file",
					},
				},
				Action: func(ctx *cli.Context) error {
					if err := clientCertAdd(ctx); err != nil {
						logging.Logger.Error(err)
					}
					return nil
				},
			},
			{
				Name: "rm",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "id",
						Usage: "the id the cert belongs to",
					},
				},
				Action: func(ctx *cli.Context) error {
					if err := clientCertRm(ctx); err != nil {
						logging.Logger.Error(err)
					}
					return nil
				},
			},
		},
	}
}

func clientCertAdd(ctx *cli.Context) error {
	logging.Logger.Infof("%s exec: client cert add", ctx.App.Version)

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
}

func clientCertRm(ctx *cli.Context) error {
	logging.Logger.Infof("%s exec: client cert rm", ctx.App.Version)

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
}
