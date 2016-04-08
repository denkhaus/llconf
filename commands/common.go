package commands

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/gob"
	"encoding/pem"
	"net"

	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"math/big"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	syslogger "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/codegangsta/cli"

	"github.com/denkhaus/llconf/compiler"
	"github.com/denkhaus/llconf/promise"
	"github.com/denkhaus/llconf/server"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
	"github.com/juju/errors"
)

type SendCommand struct {
	Data        []byte
	Stdin       io.Writer
	Stdout      io.Reader
	Stderr      io.Reader
	SendChannel libchan.Sender
}

type RunCtx struct {
	Verbose      bool
	UseSyslog    bool
	Interval     int
	Port         int
	RootPromise  string
	InputDir     string
	WorkDir      string
	RunlogPath   string
	Host         string
	SettingsDir  string
	CertDir      string
	PrivKeyFile  string
	CertFile     string
	AppCtx       *cli.Context
	AppLogger    *logrus.Logger
	Sender       libchan.Sender
	Receiver     libchan.Receiver
	RemoteSender libchan.Sender
}

func NewRunCtx(ctx *cli.Context, logger *logrus.Logger) (*RunCtx, error) {
	rCtx := RunCtx{AppCtx: ctx, AppLogger: logger}
	err := rCtx.parseArguments()
	return &rCtx, err
}

func (p *RunCtx) setupLogging() error {
	if p.UseSyslog {
		hook, err := syslogger.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err != nil {
			return err
		}

		p.AppLogger.Hooks.Add(hook)
	}

	return nil
}

func (p *RunCtx) ensureCertificate() error {

	if fileExists(p.PrivKeyFile) &&
		fileExists(p.CertFile) {
		return nil
	}

	p.AppLogger.Info("create certificates")

	template := &x509.Certificate{
		IsCA: true,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1, 2, 3},
		SerialNumber:          big.NewInt(1234),
		Subject: pkix.Name{
			Country:      []string{"Earth"},
			Organization: []string{"llconf"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(5, 5, 5),
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return errors.Annotate(err, "generate priv key")
	}

	publickey := &privatekey.PublicKey
	cert, err := x509.CreateCertificate(rand.Reader, template,
		template, publickey, privatekey)
	if err != nil {
		return errors.Annotate(err, "create certificate")
	}

	pemkey := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privatekey),
	}

	buf := pem.EncodeToMemory(pemkey)
	if err := ioutil.WriteFile(p.PrivKeyFile, buf, 0644); err != nil {
		return errors.Annotate(err, "write priv key")
	}

	pemkey = &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}

	buf = pem.EncodeToMemory(pemkey)
	if err := ioutil.WriteFile(p.CertFile, buf, 0644); err != nil {
		return errors.Annotate(err, "write cert")
	}

	return nil
}

func (p *RunCtx) createClientServer() error {
	if err := p.ensureCertificate(); err != nil {
		return errors.Annotate(err, "ensure certificate")
	}

	srv := server.New(p.Host, p.Port, p.PrivKeyFile, p.CertFile, p.AppLogger)
	srv.OnPromiseReceived = p.execPromise
	if err := srv.ListenAndRun(); err != nil {
		return errors.Annotate(err, "server listen")
	}

	conn := net.JoinHostPort(p.Host, fmt.Sprintf("%d", p.Port))
	client, err := tls.Dial("tcp", conn, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return errors.Annotate(err, "dial")
	}

	pr, err := spdy.NewSpdyStreamProvider(client, false)
	if err != nil {
		return errors.Annotate(err, "new stream provider")
	}

	transport := spdy.NewTransport(pr)
	snd, err := transport.NewSendChannel()
	if err != nil {
		return errors.Annotate(err, "new send channel")
	}

	p.Sender = snd
	p.Receiver, p.RemoteSender = libchan.Pipe()
	return nil
}

func (p *RunCtx) compilePromise() (promise.Promise, error) {
	promises, err := compiler.Compile(p.InputDir)
	if err != nil {
		return nil, errors.Errorf("parsing input folder: %v", err)
	}

	tree, ok := promises[p.RootPromise]
	if !ok {
		return nil, errors.New("root promise (" + p.RootPromise + ") unknown")
	}

	return tree, nil
}

func (p *RunCtx) parseArguments() error {
	args := p.AppCtx.Args()

	switch len(args) {
	case 0:
		p.AppLogger.Fatal("config: no workdir specified")
	case 1:
		p.WorkDir = args.First()
	default:
		p.AppLogger.Fatal("config: argument count mismatch")
	}

	p.InputDir = p.AppCtx.String("input-folder")
	if p.InputDir == "" {
		p.InputDir = filepath.Join(p.WorkDir, "input")
	}
	p.RunlogPath = p.AppCtx.String("runlog-path")
	if p.RunlogPath == "" {
		p.RunlogPath = filepath.Join(p.WorkDir, "runlog")
	}

	p.RootPromise = p.AppCtx.GlobalString("promise")
	p.Verbose = p.AppCtx.GlobalBool("verbose")
	p.Host = p.AppCtx.GlobalString("host")
	p.Port = p.AppCtx.GlobalInt("port")
	p.Interval = p.AppCtx.Int("interval")
	p.UseSyslog = p.AppCtx.Bool("syslog")

	usr, err := user.Current()
	if err != nil {
		return errors.Annotate(err, "get current user")
	}

	p.SettingsDir = path.Join(usr.HomeDir, "/.llconf")
	if err := os.MkdirAll(p.SettingsDir, 0755); err != nil {
		return errors.Annotate(err, "create settings dir")
	}
	p.CertDir = path.Join(usr.HomeDir, "/.llconf/secure")
	if err := os.MkdirAll(p.CertDir, 0755); err != nil {
		return errors.Annotate(err, "create cert dir")
	}

	p.PrivKeyFile = path.Join(p.CertDir, "privkey.pem")
	p.CertFile = path.Join(p.CertDir, "cert.pem")

	// when run as daemon, the home folder isn't set
	if os.Getenv("HOME") == "" {
		os.Setenv("HOME", p.WorkDir)
	}

	gob.Register(promise.NamedPromise{})
	gob.Register(promise.ExecPromise{})
	gob.Register(promise.Constant("const"))
	return nil
}

func (p *RunCtx) sendPromise(tree promise.Promise) error {
	cmd := SendCommand{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		SendChannel: p.RemoteSender,
	}

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tree); err != nil {
		return errors.Annotate(err, "encode")
	}

	cmd.Data = buf.Bytes()
	if err := p.Sender.Send(&cmd); err != nil {
		return errors.Annotate(err, "send")
	}

	resp := &server.CommandResponse{}
	if err := p.Receiver.Receive(&resp); err != nil {
		return errors.Annotate(err, "receive")
	}

	p.AppLogger.Info(resp.Error)
	return nil
}

func (p *RunCtx) execPromise(tree promise.Promise) {
	vars := promise.Variables{}
	vars["input_dir"] = p.InputDir
	vars["work_dir"] = p.WorkDir
	vars["executable"] = filepath.Clean(os.Args[0])

	log := promise.Logger{}
	log.Logger = p.AppLogger

	ctx := promise.Context{
		Logger:     &log,
		ExecOutput: &bytes.Buffer{},
		Vars:       vars,
		Args:       p.AppCtx.Args(),
		Env:        []string{},
		Verbose:    p.Verbose,
		InDir:      "",
	}

	starttime := time.Now().Local()
	fullfilled := tree.Eval([]promise.Constant{}, &ctx, "")
	endtime := time.Now().Local()

	p.AppLogger.Infof("%d changes and %d tests executed", ctx.Logger.Changes, ctx.Logger.Tests)
	if fullfilled {
		ctx.Logger.Infof("evaluation successful")
	} else {
		ctx.Logger.Error("error during evaluation")
	}

	writeRunLog(fullfilled, ctx.Logger.Changes,
		ctx.Logger.Tests, starttime, endtime, p.RunlogPath)
}

func writeRunLog(success bool, changes, tests int,
	starttime, endtime time.Time, path string) (err error) {
	var output string

	duration := endtime.Sub(starttime)

	if success {
		output = fmt.Sprintf("successful, endtime=%d, duration=%f, c=%d, t=%d",
			endtime.Unix(), duration.Seconds(), changes, tests)
	} else {
		output = fmt.Sprintf("error, endtime=%d, duration=%f, c=%d, t=%d",
			endtime.Unix(), duration.Seconds(), changes, tests)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	data := []byte(output)
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		return
	}

	err = f.Close()
	return
}

func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}
