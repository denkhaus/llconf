package promise

import (
	"bytes"

	"github.com/Sirupsen/logrus"
)

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
	Logger     *Logger
	ExecOutput *bytes.Buffer
	Vars       Variables
	Args       []string
	Env        []string
	InDir      string
	Verbose    bool
}

func NewContext() Context {
	return Context{
		Logger: NewLogger(),
		Vars:   make(map[string]string),
		InDir:  "",
	}
}

type Logger struct {
	*logrus.Logger
	Changes int
	Tests   int
}

func NewLogger() *Logger {
	log := Logger{}
	log.Logger = logrus.New()
	return &log
}
