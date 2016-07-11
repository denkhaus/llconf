package promise

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/juju/errors"
)

type TemplatePromise struct {
	JsonInput    Argument
	TemplateFile Argument
	Output       Argument
}

func (t TemplatePromise) New(children []Promise, args []Argument) (Promise, error) {
	if len(args) == 3 {
		return TemplatePromise{args[0], args[1], args[2]}, nil
	} else {
		return nil, errors.New("(template) has not enough arguments")
	}
}

func (t TemplatePromise) Desc(arguments []Constant) string {
	return fmt.Sprintf("(template in:%s temp:%s out:%s)",
		t.JsonInput,
		t.TemplateFile,
		t.Output)
}

func (t TemplatePromise) Eval(arguments []Constant, ctx *Context, stack string) error {
	replacer := strings.NewReplacer("'", "\"")
	json_input := replacer.Replace(t.JsonInput.GetValue(arguments, &ctx.Vars))
	template_file := t.TemplateFile.GetValue(arguments, &ctx.Vars)
	output := t.Output.GetValue(arguments, &ctx.Vars)

	var input interface{}
	if err := json.Unmarshal([]byte(json_input), &input); err != nil {
		return errors.Annotate(err, "unmarshal")
	}

	tmpl, err := template.ParseFiles(template_file)
	if err != nil {
		return errors.Annotate(err, "parse files")
	}

	fo, err := os.Create(output)
	if err != nil {
		return errors.Annotate(err, "create output file")
	}
	defer fo.Close()

	bfo := bufio.NewWriter(fo)
	if err := tmpl.Execute(bfo, input); err != nil {
		return errors.Annotate(err, "exec template")
	}

	bfo.Flush()
	return nil
}
