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
	"runtime/debug"
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
	"github.com/denkhaus/goagain"
	"github.com/denkhaus/llconf/compiler"
	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/promise"
	"github.com/denkhaus/llconf/server"
	"github.com/denkhaus/llconf/store"
	"github.com/denkhaus/llconf/util"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
	"github.com/juju/errors"
)

//////////////////////////////////////////////////////////////////////////////////
type RemoteCommand struct {
	Data          []byte
	Stdout        io.Reader
	SendChannel   libchan.Sender
	Verbose       bool
	Debug         bool
	ClientVersion string
}

//////////////////////////////////////////////////////////////////////////////////
type context struct {
	verbose            bool
	debug              bool
	useSyslog          bool
	noRedirect         bool
	port               int
	clientVersion      string
	rootPromise        string
	LibDir             string
	InputDir           string
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
func New(ctx *cli.Context, isClient bool, needInput bool) (*context, error) {
	rCtx := context{appCtx: ctx}

	err := rCtx.parseArguments(isClient, needInput)
	if err == nil && isClient {
		// only in client mode, since server has its own signal handling
		go rCtx.clientSignalHandler()
	}

	return &rCtx, err
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) Close() error {
	logging.Logger.Debug("context: close")

	if p.dataStore != nil {
		if err := p.dataStore.Close(); err != nil {
			return errors.Annotate(err, "close datastore")
		}

		logging.Logger.Info("datastore closed")
		p.dataStore = nil
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
func (p *context) clientSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGTERM,
		syscall.SIGINT)

	sig := <-sigChan
	signal.Stop(sigChan)
	logging.Logger.Infof("%s signal received", sig.String())

	if err := p.Close(); err != nil {
		logging.Logger.Error(errors.Annotate(err, "close context"))
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) upgradeLogging() error {
	if p.useSyslog {
		hook, err := syslogger.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err != nil {
			return errors.Annotate(err, "syslog hook")
		}

		logging.Logger.Hooks.Add(hook)
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ensureClientCert() error {
	if util.FileExists(p.clientPrivKeyPath) &&
		util.FileExists(p.clientCertFilePath) {
		return nil
	}

	logging.Logger.Info("create client certificates")
	return p.generateCert(p.clientPrivKeyPath, p.clientCertFilePath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ensureServerCert() error {
	if util.FileExists(p.serverPrivKeyPath) &&
		util.FileExists(p.serverCertFilePath) {
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

	hn, err := os.Hostname()
	if err != nil {
		return errors.Annotate(err, "get hostname")
	}

	template.DNSNames = append(template.DNSNames, hn)

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
func (p *context) StartServer() error {
	logging.Logger.Debug("context: start server")
	srv := server.New(
		p.host,
		p.port,
		p.dataStore,
		p.ExecPromise,
		p.noRedirect,
		p.clientVersion,
	)

	goagain.SetLogger(logging.Logger)

	// close context before forking a new process,
	// that cannot startup, because datastore is locked
	goagain.OnBeforeSIGUSR2 = func(l net.Listener) error {
		if err := p.Close(); err != nil {
			return errors.Annotate(err, "close context before forking")
		}
		return nil
	}

	cert, err := p.loadServerCert()
	if err != nil {
		return errors.Annotate(err, "load server cert")
	}

	logging.Logger.Debug("context: try to get used listener")
	list, err := goagain.Listener()

	if err != nil {
		logging.Logger.Debug("context: start with new listener")

		if err := srv.CreateListenerAndRun(cert); err != nil {
			return errors.Annotate(err, "create listener")
		}
	} else {
		logging.Logger.Debugf("context: reuse listener from parent process for %s", list.Addr())
		if err := srv.ReuseListenerAndRun(list, cert); err != nil {
			return errors.Annotate(err, "reuse listener")
		}

		logging.Logger.Debug("context: kill parent process")
		if err := goagain.Kill(); nil != err {
			return errors.Annotate(err, "kill parent")
		}
	}

	if !srv.Alive() {
		if err := srv.LastError(); err != nil {
			return errors.Annotate(err, "server died")
		}

		return nil
	}

	logging.Logger.Debug("context: wait for signals")
	if _, err := goagain.Wait(srv.ListenerTCP()); nil != err {
		return errors.Annotate(err, "goagain wait")
	}

	if err := srv.Close(); nil != err {
		return errors.Annotate(err, "server close")
	}

	time.Sleep(1 * time.Second)
	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) loadServerCert() (*tls.Certificate, error) {
	logging.Logger.Debug("context: load server certificates")

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

	promises, err := compiler.Compile(p.LibDir, p.InputDir)
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
func (p *context) parseArguments(isClient bool, needInput bool) error {

	wd, err := os.Getwd()
	if err != nil {
		return errors.Annotate(err, "get wd")
	}
	p.workDir = wd

	if isClient && needInput {
		p.InputDir = p.appCtx.GlobalString("input-folder")

		if p.InputDir == "" {
			p.InputDir = p.workDir
		} else {
			if !filepath.IsAbs(p.InputDir) {
				p.InputDir, err = filepath.Abs(p.InputDir)
				if err != nil {
					return errors.Annotate(err, "make input path absolute")
				}
			}
		}

		if !util.FileExists(p.InputDir) {
			logging.Logger.Warnf("input folder %q does not exist", p.InputDir)
			p.InputDir = p.workDir
		}

		logging.Logger.Infof("use input @ %q", p.InputDir)
	}

	p.runlogPath = p.appCtx.GlobalString("runlog-path")
	if p.runlogPath == "" {
		p.runlogPath = filepath.Join(p.workDir, "run.log")
	}

	p.clientVersion = p.appCtx.App.Version
	p.verbose = p.appCtx.Bool("verbose")
	p.debug = p.appCtx.Bool("debug")

	logging.SetDebug(p.debug)
	logging.Logger.Infof("verbose: %t debug: %t", p.verbose, p.debug)

	p.rootPromise = p.appCtx.String("promise")
	p.host = p.appCtx.GlobalString("host")
	p.port = p.appCtx.GlobalInt("port")

	p.useSyslog = p.appCtx.GlobalBool("syslog")
	if err := p.upgradeLogging(); err != nil {
		return errors.Annotate(err, "upgrade logging")
	}

	usr, err := user.Current()
	if err != nil {
		return errors.Annotate(err, "get current user")
	}

	p.settingsDir = path.Join(usr.HomeDir, ".llconf")
	if err := os.MkdirAll(p.settingsDir, 0755); err != nil {
		return errors.Annotate(err, "create settings dir")
	}

	certDir := path.Join(p.settingsDir, "cert")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return errors.Annotate(err, "create cert dir")
	}

	dataStorePath := path.Join(p.settingsDir, "store")
	if err := os.MkdirAll(dataStorePath, 0700); err != nil {
		return errors.Annotate(err, "create datastore dir")
	}

	p.LibDir = path.Join(p.settingsDir, "lib")
	if err := os.MkdirAll(p.LibDir, 0755); err != nil {
		return errors.Annotate(err, "create lib dir")
	}

	logging.Logger.Infof("use library @ %q", p.LibDir)

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

		p.noRedirect = p.appCtx.Bool("no-redirect")
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
	gob.Register(promise.TruePromise{})
	gob.Register(promise.FalsePromise{})
	gob.Register(promise.NotPromise{})
	gob.Register(promise.PipePromise{})
	gob.Register(promise.SPipePromise{})
	gob.Register(promise.EvalPromise{})
	gob.Register(promise.ArgGetter{})
	gob.Register(promise.JoinArgument{})
	gob.Register(promise.InDir{})
	gob.Register(promise.RestartPromise{})
	gob.Register(promise.SetvarPromise{})
	gob.Register(promise.ReadvarPromise{})
	gob.Register(promise.VarGetter{})
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

	if !util.FileExists(certPath) {
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
	if tree == nil {
		return errors.New("no valid promises")
	}

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tree); err != nil {
		return errors.Annotate(err, "encode")
	}

	cmd := RemoteCommand{
		Data:          buf.Bytes(),
		Stdout:        os.Stdout,
		SendChannel:   p.remoteSender,
		Verbose:       p.verbose,
		Debug:         p.debug,
		ClientVersion: p.clientVersion,
	}

	stdout := os.Stdout
	logging.Logger.Info("send promise")
	if err := p.sender.Send(cmd); err != nil {
		return errors.Annotate(err, "send")
	}

	resp := server.CommandResponse{}
	if err := p.receiver.Receive(&resp); err != nil {
		return errors.Annotate(err, "receive")
	}

	if err := p.sender.Close(); err != nil {
		return errors.Annotate(err, "close sender channel")
	}

	os.Stdout = stdout
	logging.Logger.Info(resp.Status)

	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ExecPromise(tree promise.Promise, verbose bool) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			if errs, ok := e.(*errors.Err); ok {
				err = errs
				return
			}
			err = errors.Errorf("server panic happend: %s", debug.Stack())
		}
	}()

	vars := promise.Variables{}

	vars["work_dir"] = p.workDir
	vars["settings_dir"] = p.settingsDir
	vars["lib_dir"] = p.LibDir
	vars["executable"] = filepath.Clean(os.Args[0])

	ctx := promise.Context{
		ExecOutput: &bytes.Buffer{},
		Compile:    compiler.Compile,
		Vars:       vars,
		Args:       os.Args[1:],
		Env:        []string{},
		Verbose:    verbose,
		InDir:      "",
	}

	starttime := time.Now().Local()
	res := tree.Eval([]promise.Constant{}, &ctx, "")
	endtime := time.Now().Local()

	defer logging.Logger.Reset()
	logging.Logger.Infof("%d changes and %d tests executed in %s",
		logging.Logger.Changes, logging.Logger.Tests, endtime.Sub(starttime))

	writeRunLog(res, starttime, endtime, p.runlogPath)
	return
}

//////////////////////////////////////////////////////////////////////////////////
func writeRunLog(success bool, starttime, endtime time.Time, path string) error {
	var output string

	changes := logging.Logger.Changes
	tests := logging.Logger.Tests
	duration := endtime.Sub(starttime)

	output = fmt.Sprintf("error, endtime=%d, duration=%f, c=%d, t=%d -> %t",
		endtime.Unix(), duration.Seconds(), changes, tests, success)

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
