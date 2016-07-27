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
	"github.com/rcrowley/goagain"

	"github.com/denkhaus/llconf/promise"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
	"gopkg.in/tomb.v2"
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

type oprFunc func(pr promise.Promise, verbose bool) error

//////////////////////////////////////////////////////////////////////////////////
type Server struct {
	tomb              tomb.Tomb
	listener          net.Listener
	host              string
	port              string
	dataStore         *store.DataStore
	OnPromiseReceived oprFunc
}

//////////////////////////////////////////////////////////////////////////////////
func New(host string, port int, ds *store.DataStore, opr oprFunc) *Server {
	serv := Server{
		host:              host,
		port:              fmt.Sprintf("%d", port),
		dataStore:         ds,
		OnPromiseReceived: opr,
	}

	return &serv
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) Close() error {
	defer p.listener.Close()
	p.tomb.Kill(nil)
	return p.tomb.Wait()
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) Alive() bool {
	return p.tomb.Alive()
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) LastError() error {
	return p.tomb.Err()
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) Listener() net.Listener {
	return p.listener
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) ReuseListenerAndRun(l net.Listener) error {
	logging.Logger.Infof("resume listening on %s", l.Addr())
	p.listener = l

	p.tomb.Go(func() error {
		if err := p.run(); err != nil {
			return errors.Annotate(err, "reuse run")
		}

		return nil
	})

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) CreateListenerAndRun(cert *tls.Certificate) error {
	pool, err := p.dataStore.Pool()
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
	hostPort := net.JoinHostPort(p.host, p.port)

	l, err := tls.Listen("tcp", hostPort, tlsConfig)
	if err != nil {
		return errors.Annotate(err, "listen")
	}

	logging.Logger.Infof("listening on %s", hostPort)
	p.listener = l

	p.tomb.Go(func() error {
		if err := p.run(); err != nil {
			return errors.Annotate(err, "new run")
		}

		return nil
	})

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) redirectOutput(writer io.Writer, fn func() error) error {
	defer logging.SetOutWriter(os.Stdout)
	logging.SetOutWriter(writer)
	return fn()
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

		res := CommandResponse{
			Status: "execution successfull",
		}

		err := p.redirectOutput(cmd.Stdout, func() error {
			return p.OnPromiseReceived(pr, cmd.Verbose)
		})

		if err != nil {
			res.Error = err.Error()
			res.Status = "execution aborted with error"
		}

		logging.Logger.Info("send response")
		if err := cmd.SendChannel.Send(&res); err != nil {
			return errors.Annotate(err, "send")
		}

		select {
		case <-p.tomb.Dying():
			return tomb.ErrDying
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
		p.tomb.Go(func() error {
			if err := p.receiveLoop(receiver); err != nil {
				return errors.Annotate(err, "receive loop")
			}
			return nil
		})

		select {
		case <-p.tomb.Dying():
			return tomb.ErrDying
		}
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) run() error {
	for {
		logging.Logger.Debug("server: wait for connection")

		c, err := p.listener.Accept()
		if err != nil {
			if goagain.IsErrClosing(err) {
				break
			}

			return errors.Annotate(err, "accept")
		}

		logging.Logger.Debug("server: connection available")

		pr, err := spdy.NewSpdyStreamProvider(c, true)
		if err != nil {
			return errors.Annotate(err, "new stream provider")
		}

		p.tomb.Go(func() error {
			defer pr.Close()
			t := spdy.NewTransport(pr)
			if err := p.receive(t); err != nil {
				return errors.Annotate(err, "new receive")
			}
			return nil
		})

		select {
		case <-p.tomb.Dying():
			return tomb.ErrDying
		}
	}

	return nil
}
