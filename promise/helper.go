package promise

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/juju/errors"
)

////////////////////////////////////////////////////////////////////////////////
func sanitizeInDir(ctx *Context) error {
	if ctx.InDir == "" {
		return nil
	}

	if len(ctx.InDir) >= 2 && ctx.InDir[:2] == "~/" {
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

////////////////////////////////////////////////////////////////////////////////
func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil &&
		os.IsNotExist(err) {
		return false
	}

	return true
}
