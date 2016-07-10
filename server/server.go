package server

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/juju/errors"

	"github.com/denkhaus/llconf/promise"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
)

//////////////////////////////////////////////////////////////////////////////////
type RemoteCommand struct {
	Data        []byte
	Stdin       io.Reader
	Stdout      io.WriteCloser
	Stderr      io.WriteCloser
	SendChannel libchan.Sender
}

//////////////////////////////////////////////////////////////////////////////////
type CommandResponse struct {
	Error error
}

//////////////////////////////////////////////////////////////////////////////////
type Server struct {
	listener          net.Listener
	Host              string
	Port              string
	CertFile          string
	KeyFile           string
	Logger            *logrus.Logger
	OnPromiseReceived func(promise.Promise) error
}

//////////////////////////////////////////////////////////////////////////////////
func New(host string, port int, keyFile, certFile string, logger *logrus.Logger) *Server {
	serv := Server{
		Host:     host,
		Port:     fmt.Sprintf("%d", port),
		KeyFile:  keyFile,
		CertFile: certFile,
		Logger:   logger,
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

	p.listener = list
	p.run()

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) redirectOutput(cmd *RemoteCommand, ch chan bool) {
	process := func(reader io.Reader, writer io.Writer) {
		scn := bufio.NewScanner(reader)
		for scn.Scan() {
			fmt.Fprintf(writer, "remote: %s", scn.Text())

			select {
			case <-ch:
				p.Logger.Info("redirect finished")
				return
			default:
				p.Logger.Info("4")
			}
		}
	}

	go process(os.Stdout, cmd.Stdout)
	go process(os.Stderr, cmd.Stderr)
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) receiveLoop(receiver libchan.Receiver) error {
	for {

		cmd := RemoteCommand{}
		err := receiver.Receive(&cmd)
		if err != nil {
			return errors.Annotate(err, "receive")
		}

		pr := promise.NamedPromise{}
		enc := gob.NewDecoder(bytes.NewBuffer(cmd.Data))
		if err := enc.Decode(&pr); err != nil {
			return errors.Annotate(err, "decode")
		}

		ch := make(chan bool)
		p.redirectOutput(&cmd, ch)

		if err := p.OnPromiseReceived(pr); err != nil {
			return errors.Annotate(err, "on promise received")
		}

		res := CommandResponse{}
		if err := cmd.SendChannel.Send(&res); err != nil {
			return errors.Annotate(err, "send")
		}

		ch <- true
	}
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) receive(t libchan.Transport) error {
	for {
		receiver, err := t.WaitReceiveChannel()
		if err != nil {
			return errors.Annotate(err, "wait receive channel")
		}

		go func() {
			if err := p.receiveLoop(receiver); err != nil {
				p.Logger.Fatalf("server: receive loop ended with error: %v", err)
			}
		}()
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) run() error {
	p.Logger.Info("start server")

	process := func() error {
		for {
			c, err := p.listener.Accept()
			if err != nil {
				return errors.Annotate(err, "accept")
			}
			pr, err := spdy.NewSpdyStreamProvider(c, true)
			if err != nil {
				return errors.Annotate(err, "new stream provider")
			}

			go func() {
				t := spdy.NewTransport(pr)
				if err := p.receive(t); err != nil {
					pr.Close()
					p.Logger.Fatalf("server: receive ended with error: %v", err)
				}
			}()
		}
	}

	return process()
}
