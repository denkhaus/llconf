package logging

import (
	"io"
	"os"

	"github.com/Sirupsen/logrus"
)

var Logger *stdLogger

type stdLogger struct {
	*logrus.Logger
	Changes int
	Tests   int
}

func (p *stdLogger) Reset() {
	p.Changes = 0
	p.Tests = 0
}

func init() {
	log := &stdLogger{}
	log.Logger = logrus.New()
	log.Level = logrus.DebugLevel
	log.Out = os.Stdout
	Logger = log
}

func SetOutWriter(writer io.Writer) {
	Logger.Out = writer
}
