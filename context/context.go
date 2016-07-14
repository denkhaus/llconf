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
	UseSyslog          bool
	Interval           int
	Port               int
	RootPromise        string
	InputDir           string
	WorkDir            string
	RunlogPath         string
	Host               string
	SettingsDir        string
	DataStore          *store.DataStore
	ClientPrivKeyPath  string
	ClientCertFilePath string
	ServerPrivKeyPath  string
	ServerCertFilePath string
	AppCtx             *cli.Context
	Sender             libchan.Sender
	Receiver           libchan.Receiver
	RemoteSender       libchan.Sender
}

//////////////////////////////////////////////////////////////////////////////////
func New(ctx *cli.Context, isClient bool) (*context, error) {
	rCtx := context{AppCtx: ctx}
	err := rCtx.parseArguments(isClient)
	if err == nil {
		go rCtx.signalHandler()
	}

	return &rCtx, err
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) Close() error {
	logging.Logger.Debug("close context")

	if p.DataStore != nil {
		err := p.DataStore.Close()
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
	if p.UseSyslog {
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
	if fileExists(p.ClientPrivKeyPath) &&
		fileExists(p.ClientCertFilePath) {
		return nil
	}

	logging.Logger.Info("create client certificates")
	return p.generateCert(p.ClientPrivKeyPath, p.ClientCertFilePath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ensureServerCert() error {
	if fileExists(p.ServerPrivKeyPath) &&
		fileExists(p.ServerCertFilePath) {
		return nil
	}

	logging.Logger.Info("create server certificates")
	return p.generateCert(p.ServerPrivKeyPath, p.ServerCertFilePath)
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

	srv := server.New(p.Host, p.Port, p.DataStore)
	srv.OnPromiseReceived = p.ExecPromise

	if err := srv.ListenAndRun(cert); err != nil {
		return errors.Annotate(err, "server listen")
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) loadServerCert() (*tls.Certificate, error) {
	if _, err := os.Stat(p.ServerCertFilePath); os.IsNotExist(err) {
		return nil, errors.New("tls cert file not found")
	}
	if _, err := os.Stat(p.ServerPrivKeyPath); os.IsNotExist(err) {
		return nil, errors.New("tls privkey file not found")
	}

	tlsCert, err := tls.LoadX509KeyPair(p.ServerCertFilePath, p.ServerPrivKeyPath)
	if err != nil {
		return nil, errors.Annotate(err, "load key pair")
	}

	return &tlsCert, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) loadClientCert() (*tls.Certificate, error) {
	if _, err := os.Stat(p.ClientCertFilePath); os.IsNotExist(err) {
		return nil, errors.New("tls cert file not found")
	}
	if _, err := os.Stat(p.ClientPrivKeyPath); os.IsNotExist(err) {
		return nil, errors.New("tls privkey file not found")
	}

	tlsCert, err := tls.LoadX509KeyPair(p.ClientCertFilePath, p.ClientPrivKeyPath)
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

	pool, err := p.DataStore.ServerCertPool()
	if err != nil {
		return errors.Annotate(err, "get server cert pool")
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{*cert},
		RootCAs:      pool,
	}

	tlsConfig.BuildNameToCertificate()
	hostPort := net.JoinHostPort(p.Host, fmt.Sprintf("%d", p.Port))

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

	p.Sender = snd
	p.Receiver, p.RemoteSender = libchan.Pipe()
	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) CompilePromise() (promise.Promise, error) {
	logging.Logger.Info("compile promise")

	promises, err := compiler.Compile(p.InputDir)
	if err != nil {
		return nil, errors.Annotate(err, "compile promise")
	}

	tree, ok := promises[p.RootPromise]
	if !ok {
		return nil, errors.New("root promise (" + p.RootPromise + ") unknown")
	}

	return tree, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) parseArguments(isClient bool) error {

	wd, err := os.Getwd()
	if err != nil {
		return errors.Annotate(err, "get wd")
	}
	p.WorkDir = wd

	if isClient {
		p.InputDir = p.AppCtx.String("input-folder")
		if p.InputDir == "" {
			p.InputDir = filepath.Join(p.WorkDir, "input")
		}
		if err := os.MkdirAll(p.InputDir, 0755); err != nil {
			return errors.Annotate(err, "create input dir")
		}
	}

	p.RunlogPath = p.AppCtx.String("runlog-path")
	if p.RunlogPath == "" {
		p.RunlogPath = filepath.Join(p.WorkDir, "run.log")
	}

	logging.SetDebug(p.AppCtx.GlobalBool("debug"))
	p.RootPromise = p.AppCtx.GlobalString("promise")
	p.Verbose = p.AppCtx.GlobalBool("verbose")
	p.Host = p.AppCtx.GlobalString("host")
	p.Port = p.AppCtx.GlobalInt("port")
	p.Interval = p.AppCtx.Int("interval")

	p.UseSyslog = p.AppCtx.Bool("syslog")
	if err := p.upgradeLogging(); err != nil {
		return errors.Annotate(err, "upgrade logging")
	}

	usr, err := user.Current()
	if err != nil {
		return errors.Annotate(err, "get current user")
	}

	p.SettingsDir = path.Join(usr.HomeDir, "/.llconf")
	if err := os.MkdirAll(p.SettingsDir, 0755); err != nil {
		return errors.Annotate(err, "create settings dir")
	}

	certDir := path.Join(usr.HomeDir, "/.llconf/cert")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return errors.Annotate(err, "create cert dir")
	}

	if isClient {
		p.ClientPrivKeyPath = path.Join(certDir, "client.privkey.pem")
		p.ClientCertFilePath = path.Join(certDir, "client.cert.pem")
		if err := p.ensureClientCert(); err != nil {
			return errors.Annotate(err, "ensure client cert")
		}
	} else {
		p.ServerPrivKeyPath = path.Join(certDir, "server.privkey.pem")
		p.ServerCertFilePath = path.Join(certDir, "server.cert.pem")
		if err := p.ensureServerCert(); err != nil {
			return errors.Annotate(err, "ensure server cert")
		}
	}

	dataStorePath := path.Join(usr.HomeDir, "/.llconf/store")
	if err := os.MkdirAll(dataStorePath, 0700); err != nil {
		return errors.Annotate(err, "create datastore dir")
	}

	store, err := store.New(dataStorePath)
	if err != nil {
		return errors.Annotate(err, "create data store")
	}
	p.DataStore = store

	// when run as daemon, the home folder isn't set
	if os.Getenv("HOME") == "" {
		os.Setenv("HOME", p.WorkDir)
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
func (p *context) AddClientCert(clientID string, certPath string) error {
	logging.Logger.Info("server: add client cert")

	if clientID == "" {
		return errors.New("no client id provided")
	}

	if certPath == "" {
		return errors.New("no client certificate path provided")
	}

	if !fileExists(certPath) {
		return errors.New("client certificate file does not exist")
	}

	return p.DataStore.StoreClientCert(clientID, certPath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) RemoveClientCert(clientID string) error {
	logging.Logger.Info("server: remove client cert")

	if clientID == "" {
		return errors.New("no client id provided")
	}

	return p.DataStore.RemoveClientCert(clientID)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) AddServerCert(serverID string, certPath string) error {
	logging.Logger.Info("client: add server cert")

	if serverID == "" {
		return errors.New("no server id provided")
	}

	if certPath == "" {
		return errors.New("no server certificate path provided")
	}

	if !fileExists(certPath) {
		return errors.New("server certificate file does not exist")
	}

	return p.DataStore.StoreServerCert(serverID, certPath)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) RemoveServerCert(serverID string) error {
	logging.Logger.Info("client: remove server cert")

	if serverID == "" {
		return errors.New("no server id provided")
	}

	return p.DataStore.RemoveServerCert(serverID)
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
		SendChannel: p.RemoteSender,
		Verbose:     p.Verbose,
	}

	logging.Logger.Info("send promise")
	if err := p.Sender.Send(cmd); err != nil {
		return errors.Annotate(err, "send")
	}

	if err := p.Sender.Close(); err != nil {
		return errors.Annotate(err, "close sender channel")
	}

	resp := server.CommandResponse{}
	if err := p.Receiver.Receive(&resp); err != nil {
		return errors.Annotate(err, "receive")
	}

	logging.Logger.Info(resp.Status)
	return resp.Error
}

//////////////////////////////////////////////////////////////////////////////////
func (p *context) ExecPromise(tree promise.Promise, verbose bool) error {
	vars := promise.Variables{}
	vars["input_dir"] = p.InputDir
	vars["work_dir"] = p.WorkDir
	vars["executable"] = filepath.Clean(os.Args[0])

	ctx := promise.Context{
		ExecOutput: &bytes.Buffer{},
		Vars:       vars,
		Args:       p.AppCtx.Args(),
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

	writeRunLog(err, starttime, endtime, p.RunlogPath)
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
