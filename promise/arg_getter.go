package promise

import (
	"fmt"
	"io"
	"strconv"
)

type ArgGetter struct {
	Position int
}

func (p ArgGetter) GetValue(arguments []Constant, vars *Variables) string {
	if len(arguments) <= p.Position {
		return ""
	}
	return string(arguments[p.Position])
}

func (p ArgGetter) String() string {
	return "arg->" + strconv.Itoa(p.Position)
}

func (p ArgGetter) Marshal(writer io.Writer) error {
	if _, err := fmt.Fprintln(writer, p.Position); err != nil {
		return err
	}

	return nil
}
