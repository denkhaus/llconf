package promise

import (
	"fmt"
	"io"
	"os"
)

type EnvGetter struct {
	Name string
}

func (envGetter EnvGetter) GetValue(arguments []Constant, vars *Variables) string {
	value := os.Getenv(envGetter.Name)
	return value
}

func (envGetter EnvGetter) String() string {
	return "env->$" + envGetter.Name + "(" + os.Getenv(envGetter.Name) + ")"
}

func (p EnvGetter) Marshal(writer io.Writer) error {
	if _, err := fmt.Fprintln(writer, p.Name); err != nil {
		return err
	}

	return nil
}
