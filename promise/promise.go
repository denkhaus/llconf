package promise

import (
	"bytes"
	"syscall"
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

func NewContext() Context {
	return Context{
		Vars:  make(map[string]string),
		InDir: "",
	}
}
