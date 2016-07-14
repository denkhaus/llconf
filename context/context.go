package context

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/gob"
	"encoding/pem"
	"math/big"
	"net"
	"syscall"

	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"time"

	syslogger "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/codegangsta/cli"
	"github.com/denkhaus/llconf/compiler"
	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/promise"
	"github.com/denkhaus/llconf/server"
	"github.com/denkhaus/llconf/store"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
	"github.com/juju/errors"
)

//////////////////////////////////////////////////////////////////////////////////
type RemoteCommand struct {
	Data        []byte
	Stdout      io.Reader
	SendChannel libchan.Sender
	Verbose     bool
}

//////////////////////////////////////////////////////////////////////////////////
type context struct {
	Verbose            bool
	useSyslog          bool
	Interval           int
	port               int
	rootPromise        string
	inputDir           string
	workDir            string
	runlogPath         string
	host               string
	settingsDir        string
	dataStore          *store.DataStore
	clientPrivKeyPath  string
	clientCertFilePath string
	serverPrivKeyPath  string
	serverCertFilePath string
	certRole           string
	appCtx             *cli.Context
	sender             libchan.Sender
	receiver           libchan.Receiver
	remoteSender       libchan.Sender
}

//////////////////////////////////////////////////////////////////////////////////
func New(ctx *cli.Context, isClient bool) (*context, error) {
	rCtx := context{appCtx: ctx}
	err := rCtx.parseArguments(isClient)
	if err == nil {
		go rCtx.signalHandler()
	}

	return &rCtx, err
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) Close() error {
	logging.Logger.Debug("close context")

	if p.dataStore != nil {
		err := p.dataStore.Close()
		if err == nil {
			logging.Logger.Info("datastore closed")
		}
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
func (p *context) signalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	<-sigChan

	if err := p.Close(); err != nil {
		logging.Logger.Error(errors.Annotate(err, "close context"))
	}
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) upgradeLogging() error {
	if p.useSyslog {
		hook, err := syslogger.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err != nil {
			return err
		}

		logging.Logger.Hooks.Add(hook)
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ensureClientCert() error {
	if fileExists(p.clientPrivKeyPath) &&
		fileExists(p.clientCertFilePath) {
		return nil
	}

	logging.Logger.Info("create client certificates")
	return p.generateCert(p.clientPrivKeyPath, p.clientCertFilePath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ensureServerCert() error {
	if fileExists(p.serverPrivKeyPath) &&
		fileExists(p.serverCertFilePath) {
		return nil
	}

	logging.Logger.Info("create server certificates")
	return p.generateCert(p.serverPrivKeyPath, p.serverCertFilePath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) generateCert(privKeyPath string, certFilePath string) error {

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return errors.Annotate(err, "failed to generate serial number")
	}

	template := &x509.Certificate{
		IsCA: true,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject: pkix.Name{
			Country:      []string{"worldwide"},
			Organization: []string{"llconf"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(5, 5, 5),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	template.DNSNames = append(template.DNSNames, "localhost")
	template.EmailAddresses = append(template.EmailAddresses, "user@email.com")

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
	if err := ioutil.WriteFile(privKeyPath, buf, 0644); err != nil {
		return errors.Annotate(err, "write priv key")
	}

	pemkey = &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}

	buf = pem.EncodeToMemory(pemkey)
	if err := ioutil.WriteFile(certFilePath, buf, 0644); err != nil {
		return errors.Annotate(err, "write cert")
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) CreateServer() error {
	cert, err := p.loadServerCert()
	if err != nil {
		return errors.Annotate(err, "load server cert")
	}

	srv := server.New(p.host, p.port, p.dataStore)
	srv.OnPromiseReceived = p.ExecPromise

	if err := srv.ListenAndRun(cert); err != nil {
		return errors.Annotate(err, "server listen")
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) loadServerCert() (*tls.Certificate, error) {
	if _, err := os.Stat(p.serverCertFilePath); os.IsNotExist(err) {
		return nil, errors.New("tls cert file not found")
	}
	if _, err := os.Stat(p.serverPrivKeyPath); os.IsNotExist(err) {
		return nil, errors.New("tls privkey file not found")
	}

	tlsCert, err := tls.LoadX509KeyPair(p.serverCertFilePath, p.serverPrivKeyPath)
	if err != nil {
		return nil, errors.Annotate(err, "load key pair")
	}

	return &tlsCert, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) loadClientCert() (*tls.Certificate, error) {
	if _, err := os.Stat(p.clientCertFilePath); os.IsNotExist(err) {
		return nil, errors.New("tls cert file not found")
	}
	if _, err := os.Stat(p.clientPrivKeyPath); os.IsNotExist(err) {
		return nil, errors.New("tls privkey file not found")
	}

	tlsCert, err := tls.LoadX509KeyPair(p.clientCertFilePath, p.clientPrivKeyPath)
	if err != nil {
		return nil, errors.Annotate(err, "load key pair")
	}

	return &tlsCert, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) CreateClient() error {
	cert, err := p.loadClientCert()
	if err != nil {
		return errors.Annotate(err, "load client cert")
	}

	pool, err := p.dataStore.Pool()
	if err != nil {
		return errors.Annotate(err, "get server cert pool")
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{*cert},
		RootCAs:      pool,
	}

	tlsConfig.BuildNameToCertificate()
	hostPort := net.JoinHostPort(p.host, fmt.Sprintf("%d", p.port))

	conn, err := tls.Dial("tcp", hostPort, &tlsConfig)

	if err != nil {
		return errors.Annotate(err, "dial")
	}

	pr, err := spdy.NewSpdyStreamProvider(conn, false)
	if err != nil {
		return errors.Annotate(err, "new stream provider")
	}

	transport := spdy.NewTransport(pr)
	snd, err := transport.NewSendChannel()
	if err != nil {
		return errors.Annotate(err, "new send channel")
	}

	p.sender = snd
	p.receiver, p.remoteSender = libchan.Pipe()
	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) CompilePromise() (promise.Promise, error) {
	logging.Logger.Info("compile promise")

	promises, err := compiler.Compile(p.inputDir)
	if err != nil {
		return nil, errors.Annotate(err, "compile promise")
	}

	tree, ok := promises[p.rootPromise]
	if !ok {
		return nil, errors.New("root promise (" + p.rootPromise + ") unknown")
	}

	return tree, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) parseArguments(isClient bool) error {

	wd, err := os.Getwd()
	if err != nil {
		return errors.Annotate(err, "get wd")
	}
	p.workDir = wd

	if isClient {
		p.inputDir = p.appCtx.String("input-folder")
		if p.inputDir == "" {
			p.inputDir = filepath.Join(p.workDir, "input")
		}
		if err := os.MkdirAll(p.inputDir, 0755); err != nil {
			return errors.Annotate(err, "create input dir")
		}
	}

	p.runlogPath = p.appCtx.GlobalString("runlog-path")
	if p.runlogPath == "" {
		p.runlogPath = filepath.Join(p.workDir, "run.log")
	}

	logging.SetDebug(p.appCtx.GlobalBool("debug"))
	p.rootPromise = p.appCtx.GlobalString("promise")
	p.Verbose = p.appCtx.GlobalBool("verbose")
	p.host = p.appCtx.GlobalString("host")
	p.port = p.appCtx.GlobalInt("port")
	p.Interval = p.appCtx.Int("interval")

	p.useSyslog = p.appCtx.GlobalBool("syslog")
	if err := p.upgradeLogging(); err != nil {
		return errors.Annotate(err, "upgrade logging")
	}

	usr, err := user.Current()
	if err != nil {
		return errors.Annotate(err, "get current user")
	}

	p.settingsDir = path.Join(usr.HomeDir, "/.llconf")
	if err := os.MkdirAll(p.settingsDir, 0755); err != nil {
		return errors.Annotate(err, "create settings dir")
	}

	certDir := path.Join(usr.HomeDir, "/.llconf/cert")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return errors.Annotate(err, "create cert dir")
	}

	dataStorePath := path.Join(usr.HomeDir, "/.llconf/store")
	if err := os.MkdirAll(dataStorePath, 0700); err != nil {
		return errors.Annotate(err, "create datastore dir")
	}

	if isClient {
		p.clientPrivKeyPath = path.Join(certDir, "client.privkey.pem")
		p.clientCertFilePath = path.Join(certDir, "client.cert.pem")
		if err := p.ensureClientCert(); err != nil {
			return errors.Annotate(err, "ensure client cert")
		}

		p.certRole = "server"
		store, err := store.New("client", p.certRole, dataStorePath)
		if err != nil {
			return errors.Annotate(err, "create data store")
		}
		p.dataStore = store
	} else {
		p.serverPrivKeyPath = path.Join(certDir, "server.privkey.pem")
		p.serverCertFilePath = path.Join(certDir, "server.cert.pem")
		if err := p.ensureServerCert(); err != nil {
			return errors.Annotate(err, "ensure server cert")
		}

		p.certRole = "client"
		store, err := store.New("server", p.certRole, dataStorePath)
		if err != nil {
			return errors.Annotate(err, "create data store")
		}
		p.dataStore = store
	}

	// when run as daemon, the home folder isn't set
	if os.Getenv("HOME") == "" {
		os.Setenv("HOME", p.workDir)
	}

	gob.Register(promise.NamedPromise{})
	gob.Register(promise.ExecPromise{})
	gob.Register(promise.AndPromise{})
	gob.Register(promise.OrPromise{})
	gob.Register(promise.NotPromise{})
	gob.Register(promise.PipePromise{})
	gob.Register(promise.ArgGetter{})
	gob.Register(promise.JoinArgument{})
	gob.Register(promise.Constant("const"))

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) AddCert(id string, certPath string) error {
	logging.Logger.Infof("add %s cert", p.certRole)

	if id == "" {
		return errors.Errorf("no %s id provided", p.certRole)
	}

	if certPath == "" {
		return errors.Errorf("no %s certificate path provided", p.certRole)
	}

	if !fileExists(certPath) {
		return errors.Errorf("%s certificate file does not exist", p.certRole)
	}

	return p.dataStore.StoreCert(id, certPath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) RemoveCert(id string) error {
	logging.Logger.Infof("remove %s cert", p.certRole)

	if id == "" {
		return errors.Errorf("no %s id provided", p.certRole)
	}

	return p.dataStore.RemoveCert(id)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) SendPromise(tree promise.Promise) error {

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tree); err != nil {
		return errors.Annotate(err, "encode")
	}

	cmd := RemoteCommand{
		Data:        buf.Bytes(),
		Stdout:      os.Stdout,
		SendChannel: p.remoteSender,
		Verbose:     p.Verbose,
	}

	logging.Logger.Info("send promise")
	if err := p.sender.Send(cmd); err != nil {
		return errors.Annotate(err, "send")
	}

	if err := p.sender.Close(); err != nil {
		return errors.Annotate(err, "close sender channel")
	}

	resp := server.CommandResponse{}
	if err := p.receiver.Receive(&resp); err != nil {
		return errors.Annotate(err, "receive")
	}

	logging.Logger.Info(resp.Status)

	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ExecPromise(tree promise.Promise, verbose bool) error {
	vars := promise.Variables{}
	vars["input_dir"] = p.inputDir
	vars["work_dir"] = p.workDir
	vars["executable"] = filepath.Clean(os.Args[0])

	ctx := promise.Context{
		ExecOutput: &bytes.Buffer{},
		Vars:       vars,
		Args:       p.appCtx.Args(),
		Env:        []string{},
		Verbose:    verbose,
		InDir:      "",
	}

	starttime := time.Now().Local()
	err := tree.Eval([]promise.Constant{}, &ctx, "")
	endtime := time.Now().Local()

	defer logging.Logger.Reset()
	logging.Logger.Infof("%d changes and %d tests executed in %s",
		logging.Logger.Changes, logging.Logger.Tests, endtime.Sub(starttime))

	writeRunLog(err, starttime, endtime, p.runlogPath)
	return err
}

//////////////////////////////////////////////////////////////////////////////////
func writeRunLog(err error, starttime, endtime time.Time, path string) error {
	var output string

	changes := logging.Logger.Changes
	tests := logging.Logger.Tests
	duration := endtime.Sub(starttime)

	if err != nil {
		output = fmt.Sprintf("error, endtime=%d, duration=%f, c=%d, t=%d -> %s",
			endtime.Unix(), duration.Seconds(), changes, tests, err)
	} else {
		output = fmt.Sprintf("successful, endtime=%d, duration=%f, c=%d, t=%d",
			endtime.Unix(), duration.Seconds(), changes, tests)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return errors.Annotate(err, "open file")
	}

	defer f.Close()

	_, err = f.Write([]byte(output))
	if err != nil {
		return errors.Annotate(err, "write")
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}
