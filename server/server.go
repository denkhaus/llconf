package server

import (
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/store"
	"github.com/juju/errors"

	"github.com/denkhaus/llconf/promise"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
)

//////////////////////////////////////////////////////////////////////////////////
type RemoteCommand struct {
	Data        []byte
	Stdout      io.WriteCloser
	SendChannel libchan.Sender
	Verbose     bool
}

//////////////////////////////////////////////////////////////////////////////////
type CommandResponse struct {
	Status string
	Error  string
}

//////////////////////////////////////////////////////////////////////////////////
type Server struct {
	listener          net.Listener
	Host              string
	Port              string
	DataStore         *store.DataStore
	OnPromiseReceived func(pr promise.Promise, verbose bool) bool
}

//////////////////////////////////////////////////////////////////////////////////
func New(host string, port int, ds *store.DataStore) *Server {
	serv := Server{
		Host:      host,
		Port:      fmt.Sprintf("%d", port),
		DataStore: ds,
	}

	return &serv
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) ListenAndRun(cert *tls.Certificate) error {
	pool, err := p.DataStore.Pool()
	if err != nil {
		return errors.Annotate(err, "get client cert pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		// Reject any TLS certificate that cannot be validated
		ClientAuth: tls.RequireAndVerifyClientCert,
		// Ensure that we only use our "CA" to validate certificates
		ClientCAs: pool,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		// Force it server side
		PreferServerCipherSuites: true,
		// TLS 1.2 because we can
		MinVersion: tls.VersionTLS12,
	}

	tlsConfig.BuildNameToCertificate()
	hostPort := net.JoinHostPort(p.Host, p.Port)

	list, err := tls.Listen("tcp", hostPort, tlsConfig)
	if err != nil {
		return errors.Annotate(err, "listen")
	}

	logging.Logger.Infof("listening on %s", hostPort)

	p.listener = list
	return p.run()
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) redirectOutput(writer io.Writer, fn func()) {
	defer logging.SetOutWriter(os.Stdout)
	logging.SetOutWriter(writer)
	fn()
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) receiveLoop(receiver libchan.Receiver) error {
	for {

		cmd := RemoteCommand{}
		if err := receiver.Receive(&cmd); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Annotate(err, "receive")
		}

		logging.Logger.Info("promise received")

		pr := promise.NamedPromise{}
		enc := gob.NewDecoder(bytes.NewBuffer(cmd.Data))
		if err := enc.Decode(&pr); err != nil {
			return errors.Annotate(err, "decode")
		}

		res := CommandResponse{}
		p.redirectOutput(cmd.Stdout, func() {
			if p.OnPromiseReceived(pr, cmd.Verbose) {
				res.Status = "execution successfull"
			} else {
				res.Status = "execution error"
			}
		})

		logging.Logger.Info("send response")
		if err := cmd.SendChannel.Send(&res); err != nil {
			return errors.Annotate(err, "send")
		}
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) receive(t libchan.Transport) error {
	for {

		logging.Logger.Debug("server: wait for receive channel")
		receiver, err := t.WaitReceiveChannel()
		if err != nil {
			return errors.Annotate(err, "wait receive channel")
		}

		logging.Logger.Debug("server: receive channel available")
		go func() {
			if err := p.receiveLoop(receiver); err != nil {
				logging.Logger.Errorf("server: receive loop ended : %v", err)
			}
		}()
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) run() error {
	process := func() error {
		for {
			logging.Logger.Debug("server: wait for connection")

			c, err := p.listener.Accept()
			if err != nil {
				return errors.Annotate(err, "accept")
			}

			logging.Logger.Debug("server: connection available")
			pr, err := spdy.NewSpdyStreamProvider(c, true)
			if err != nil {
				return errors.Annotate(err, "new stream provider")
			}
			defer pr.Close()

			go func() {
				t := spdy.NewTransport(pr)
				if err := p.receive(t); err != nil {
					logging.Logger.Errorf("server: receive ended with error: %v", err)
				}
			}()
		}
	}

	return process()
}
