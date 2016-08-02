package promise

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"

	"github.com/juju/errors"
)

type AsUser struct {
	UserName string
	Promise  Promise
}

func (p AsUser) Desc(arguments []Constant) string {
	return fmt.Sprintf("(asuser %s %s)", p.UserName, p.Promise.Desc(arguments))
}

func (p AsUser) Eval(arguments []Constant, ctx *Context, stack string) bool {
	copyied_ctx := *ctx

	u, err := user.Lookup(p.UserName)
	if err != nil {
		panic(errors.Annotate(err, "lookup user"))
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		panic(errors.Annotate(err, "convert user id"))
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		panic(errors.Annotate(err, "convert group id"))
	}

	cred := syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}

	copyied_ctx.Credential = &cred
	return p.Promise.Eval(arguments, &copyied_ctx, stack)
}

func (p AsUser) New(children []Promise, args []Argument) (Promise, error) {

	if len(args) != 1 {
		return nil, fmt.Errorf("(asuser) needs exactly on argument, found %d", len(args))
	}

	if len(children) != 1 {
		return nil, fmt.Errorf("(asuser) needs exactly on child promise, found %d", len(children))
	}

	return AsUser{args[0].String(), children[0]}, nil
}
