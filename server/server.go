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
	Error  error
}

//////////////////////////////////////////////////////////////////////////////////
type Server struct {
	listener          net.Listener
	Host              string
	Port              string
	CertFile          string
	KeyFile           string
	OnPromiseReceived func(pr promise.Promise, verbose bool) error
}

//////////////////////////////////////////////////////////////////////////////////
func New(host string, port int, keyFile, certFile string) *Server {
	serv := Server{
		Host:     host,
		Port:     fmt.Sprintf("%d", port),
		KeyFile:  keyFile,
		CertFile: certFile,
	}

	return &serv
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) loadCertificate() (*tls.Certificate, error) {
	if _, err := os.Stat(p.CertFile); os.IsNotExist(err) {
		return nil, errors.New("tls cert file not found")
	}
	if _, err := os.Stat(p.KeyFile); os.IsNotExist(err) {
		return nil, errors.New("tls key file not found")
	}
	tlsCert, err := tls.LoadX509KeyPair(p.CertFile, p.KeyFile)
	if err != nil {
		return nil, errors.Annotate(err, "load key pair")
	}

	return &tlsCert, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) ListenAndRun() error {
	tlsCert, err := p.loadCertificate()
	if err != nil {
		return errors.Annotate(err, "load certificate")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{*tlsCert},
	}

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

		res := CommandResponse{}
		err := p.redirectOutput(cmd.Stdout, func() error {
			if err := p.OnPromiseReceived(pr, cmd.Verbose); err != nil {
				res.Error = err
				res.Status = "execution error"
				return err
			} else {
				res.Status = "execution successfull"
				return nil
			}
		})

		if err != nil {
			logging.Logger.Error(err)
		} else {
			logging.Logger.Infof(res.Status)
		}

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
