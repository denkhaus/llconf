package logging

import (
	"io"
	"os"

	"github.com/Sirupsen/logrus"
)

var Logger *stdLogger

type stdLogger struct {
	*logrus.Logger
	Changes  int
	Tests    int
	Errors   int
	Warnings int
}

func (p *stdLogger) Reset() {
	p.Changes = 0
	p.Tests = 0
}

func init() {
	log := &stdLogger{}
	log.Logger = logrus.New()
	fmt := log.Formatter.(*logrus.TextFormatter)
	fmt.ForceColors = true
	fmt.DisableSorting = true
	fmt.DisableTimestamp = true
	log.Out = os.Stdout

	Logger = log
}

func SetOutWriter(writer io.Writer) {
	Logger.Out = writer
}

func SetDebug(enabled bool) {
	if enabled {
		Logger.Level = logrus.DebugLevel
	} else {
		Logger.Level = logrus.InfoLevel
	}
}
