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

var symlinks = make(map[string]string)

func Compile(folders ...string) (map[string]promise.Promise, error) {
	wg := &sync.WaitGroup{}
	ch := make(chan string)

	for _, folder := range folders {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			listFiles(f, "cnf", ch)
		}(folder)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	inputs := []parser.Input{}

	for filename := range ch {
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		inputs = append(inputs, parser.Input{
			File:   filename,
			String: string(content),
		})
	}

	return parser.Parse(inputs)
}

func listFiles(folder, suffix string, filename chan<- string) {
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		sym, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}

		if sym != path {
			if _, ok := symlinks[sym]; !ok {
				symlinks[sym] = path
				listFiles(sym, suffix, filename)
				return nil
			}
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
