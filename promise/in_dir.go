package promise

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/juju/errors"
)

////////////////////////////////////////////////////////////////////////////////
type InDir struct {
	Dir     Argument
	Promise Promise
}

////////////////////////////////////////////////////////////////////////////////
func (p InDir) Desc(arguments []Constant) string {
	return fmt.Sprintf("(indir %s %s)", p.Dir, p.Promise.Desc(arguments))
}

////////////////////////////////////////////////////////////////////////////////
func (p InDir) Eval(arguments []Constant, ctx *Context, stack string) bool {
	inDir := p.Dir.GetValue(arguments, &ctx.Vars)

	copyied_ctx := *ctx
	if ctx.InDir != nil {
		if path.IsAbs(indir) {
			copyied_ctx.InDir = inDir
		} else {
			copyied_ctx.InDir = path.Join(ctx.InDir, indir)
		}

	} else {
		copyied_ctx.InDir = inDir
	}

	if err := sanitizeInDir(&copyied_ctx); err != nil {
		return nil, errors.Annotate(err, "sanitize indir")
	}

	return p.Promise.Eval(arguments, &copyied_ctx, stack)
}

////////////////////////////////////////////////////////////////////////////////
func (p InDir) New(children []Promise, args []Argument) (Promise, error) {

	if len(args) != 1 {
		return nil, fmt.Errorf("(indir) needs exactly on argument, found %d", len(args))
	}

	if len(children) != 1 {
		return nil, fmt.Errorf("(indir) needs exactly on child promise, found %d", len(children))
	}

	return InDir{args[0], children[0]}, nil
}

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
