package promise

import (
	"fmt"
	"io"
)

type Variables map[string]string

type VarGetter struct {
	Name string
}

func (getter VarGetter) String() string {
	return "[var:" + getter.Name + "]"
}

func (getter VarGetter) GetValue(arguments []Constant, vars *Variables) string {
	if v, present := (*vars)[getter.Name]; present {
		return v
	} else {
		return "missing"
	}
}

func (p VarGetter) Marshal(writer io.Writer) error {
	if _, err := fmt.Fprintln(writer, p.Name); err != nil {
		return err
	}

	return nil
}
