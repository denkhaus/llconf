package compiler

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/denkhaus/llconf/compiler/parser"
	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/promise"
	"github.com/juju/errors"
)

func Compile(folders ...string) (map[string]promise.Promise, error) {
	wg := &sync.WaitGroup{}
	ch := make(chan string)

	for _, folder := range folders {
		wg.Add(1)
		go listFiles(folder, "cnf", ch, wg)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	inputs := []parser.Input{}

	for filename := range ch {
		if content, err := ioutil.ReadFile(filename); err != nil {
			return nil, err
		} else {
			inputs = append(inputs, parser.Input{
				File:   filename,
				String: string(content)})
		}
	}

	return parser.Parse(inputs)
}

func listFiles(folder, suffix string, filename chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), suffix) {
			filename <- path
		}
		return nil
	})

	if err != nil {
		logging.Logger.Error(errors.Annotate(err, "walk files"))
	}
}
