package promise

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestSetEnvNew(t *testing.T) {
	old := SetEnv{}
	name := Constant("setenv")
	value := Constant("test")

	d, err := old.New([]Promise{DummyPromise{}}, []Argument{name, value})

	if err != nil {
		t.Errorf("(setenv) TestNew: %s", err.Error())
	} else {
		if d.(SetEnv).Name != name {
			t.Errorf("(setenv) TestNew: env name not set")
		}
	}
}

func TestSetNewEval(t *testing.T) {
	arguments := []Argument{
		Constant("/bin/bash"),
		Constant("-c"),
		Constant("echo $setenv"),
	}
	exec := ExecPromise{ExecTest, arguments}

	var sout bytes.Buffer
	ctx := NewContext()
	ctx.ExecOutput = &sout

	name := "setenv"
	value := "blafasel"
	s := SetEnv{Constant(name), Constant(value), exec}

	oldenv := fmt.Sprintf("%v", os.Environ())
	s.Eval([]Constant{}, &ctx, "setenv")
	newenv := fmt.Sprintf("%v", os.Environ())

	if oldenv != newenv {
		t.Errorf("(setenv) changed overall environment")
	}

	if sout.String() != "blafasel\n" {
		t.Errorf("env name not present during execution")
	}
}
