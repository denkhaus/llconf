package server

import (
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

type RemoteCommand struct {
	Data        []byte
	Stdin       io.Reader
	Stdout      io.WriteCloser
	Stderr      io.WriteCloser
	SendChannel libchan.Sender
}

type CommandResponse struct {
	Error error
}

type Server struct {
	listener          net.Listener
	Host              string
	Port              string
	CertFile          string
	KeyFile           string
	Logger            *logrus.Logger
	OnPromiseReceived func(promise.Promise)
}

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

func (p *Server) ensureValidCert() (*tls.Certificate, error) {
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

func (p *Server) ListenAndRun() error {
	tlsCert, err := p.ensureValidCert()
	if err != nil {
		return errors.Annotate(err, "ensure cert")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{*tlsCert},
	}

	conn := net.JoinHostPort(p.Host, p.Port)
	list, err := tls.Listen("tcp", conn, tlsConfig)
	if err != nil {
		return errors.Annotate(err, "listen")
	}

	p.listener = list
	p.run()

	return nil
}

//func (p *Server) redirectOutput(cmd *RemoteCommand, ch chan bool) error {
//	process := func(reader io.Reader, writer io.Writer) {
//		scn := bufio.NewScanner(reader)
//		for scn.Scan() {
//			//select {
//			//case <-ch:
//			//	p.Logger.Info("46")
//			//	return
//			//default:
//			//}
//			//p.Logger.Info("4")
//			//if  {
//			fmt.Fprintf(writer, "remote: %s", scn.Text())
//			//}
//		}
//	}

//	go process(os.Stdout, cmd.Stdout)
//	go process(os.Stderr, cmd.Stderr)
//	return nil
//}

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
		if err := p.redirectOutput(&cmd, ch); err != nil {
			return errors.Annotate(err, "redirect output")
		}

		p.OnPromiseReceived(pr)

		res := CommandResponse{}
		if err := cmd.SendChannel.Send(&res); err != nil {
			return errors.Annotate(err, "send")
		}

		//ch <- true
		//ch <- true
	}
}

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

func (p *Server) run() {
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

	go func() {
		if err := process(); err != nil {
			p.Logger.Fatalf("server: run ended with error: %v", err)
		}
	}()
}
