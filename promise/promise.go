package promise

import (
	"bytes"
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/juju/errors"
)

type compileFunc func(folders ...string) (map[string]Promise, error)

type Promise interface {
	Desc(arguments []Constant) string
	Eval(arguments []Constant, ctx *Context, stack string) bool
	New(children []Promise, args []Argument) (Promise, error)
}

type Argument interface {
	GetValue(arguments []Constant, vars *Variables) string
	String() string
}

type Context struct {
	Compile    compileFunc
	ExecOutput *bytes.Buffer
	Credential *syscall.Credential
	Vars       Variables
	Args       []string
	Env        []string
	InDir      string
	Verbose    bool
}

////////////////////////////////////////////////////////////////////////////////
func (p *Context) sanitizeInDir() error {
	if p.InDir == "" {
		return nil
	}

	if len(p.InDir) >= 2 && p.InDir[:2] == "~/" {
		usr, err := user.Current()
		if err != nil {
			return errors.Annotate(err, "get current user")
		}
		p.InDir = filepath.Join(usr.HomeDir, p.InDir[2:])
	}

	abs, err := filepath.Abs(p.InDir)
	if err != nil {
		return errors.Annotate(err, "make indir path absolute")
	}
	p.InDir = abs

	fs, err := os.Stat(p.InDir)
	if err != nil {
		return errors.Errorf("(indir) error for path %q: %s", p.InDir, err)
	}

	if !fs.IsDir() {
		return errors.Errorf("(indir) not a directory: %q", p.InDir)
	}
	return nil
}

func NewContext() Context {
	return Context{
		Vars:  make(map[string]string),
		InDir: "",
	}
}
