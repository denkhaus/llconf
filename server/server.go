package server

import (
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/denkhaus/goagain"
	"github.com/denkhaus/llconf/logging"
	"github.com/denkhaus/llconf/store"
	"github.com/denkhaus/llconf/util"
	"github.com/juju/errors"

	"github.com/denkhaus/llconf/promise"
	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
	"gopkg.in/tomb.v2"
)

//////////////////////////////////////////////////////////////////////////////////
type RemoteCommand struct {
	Data          []byte
	Stdout        io.WriteCloser
	SendChannel   libchan.Sender
	Verbose       bool
	Debug         bool
	ClientVersion string
}

//////////////////////////////////////////////////////////////////////////////////
type CommandResponse struct {
	ServerVersion string
	Status        string
	Error         string
}

type oprFunc func(pr promise.Promise, verbose bool) error

//////////////////////////////////////////////////////////////////////////////////
type Server struct {
	tomb              tomb.Tomb
	tslListener       net.Listener
	tcpListener       net.Listener
	host              string
	port              string
	serverVersion     string
	noRedirect        bool
	dataStore         *store.DataStore
	OnPromiseReceived oprFunc
}

//////////////////////////////////////////////////////////////////////////////////
func New(host string, port int, ds *store.DataStore, opr oprFunc, noRedirect bool, serverVersion string) *Server {
	serv := Server{
		host:              host,
		port:              fmt.Sprintf("%d", port),
		dataStore:         ds,
		noRedirect:        noRedirect,
		serverVersion:     serverVersion,
		OnPromiseReceived: opr,
	}

	return &serv
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) Close() error {
	logging.Logger.Debug("server: close")

	defer p.tslListener.Close()
	p.tomb.Kill(nil)

	logging.Logger.Debug("server: wait")
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
func (p *Server) ListenerTCP() net.Listener {
	return p.tcpListener
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) listenTCP(addr string) (*util.TimeoutListener, error) {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, errors.Annotate(err, "resolve tcp addr")
	}

	l, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return nil, errors.Annotate(err, "listen tcp")
	}

	return util.NewTimeoutListener(l), nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) ReuseListenerAndRun(ln net.Listener, cert *tls.Certificate) error {
	logging.Logger.Infof("resume listening on %s", ln.Addr())

	tlsConfig, err := p.prepeareTLSConfig(cert)
	if err != nil {
		return errors.Annotate(err, "prepare tls config")
	}

	p.tcpListener = util.NewTimeoutListener(ln.(*net.TCPListener))
	p.tslListener = tls.NewListener(p.tcpListener, tlsConfig)

	p.tomb.Go(func() error {
		if err := p.run(); err != nil {
			if err != tomb.ErrDying {
				return errors.Annotate(err, "reuse run")
			}
		}

		return nil
	})

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) CreateListenerAndRun(cert *tls.Certificate) error {
	tlsConfig, err := p.prepeareTLSConfig(cert)
	if err != nil {
		return errors.Annotate(err, "prepare tls config")
	}

	hostPort := net.JoinHostPort(p.host, p.port)
	ln, err := p.listenTCP(hostPort)
	if err != nil {
		return errors.Annotate(err, "listen tcp")
	}

	p.tcpListener = ln
	p.tslListener = tls.NewListener(p.tcpListener, tlsConfig)

	logging.Logger.Infof("listening on %s", hostPort)
	p.tomb.Go(func() error {
		if err := p.run(); err != nil {
			if err != tomb.ErrDying {
				return errors.Annotate(err, "new run")
			}
		}

		return nil
	})

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func (p *Server) prepeareTLSConfig(cert *tls.Certificate) (*tls.Config, error) {
	pool, err := p.dataStore.Pool()
	if err != nil {
		return nil, errors.Annotate(err, "get client cert pool")
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
	return tlsConfig, nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) redirectOutput(writer io.Writer, fn func() error) error {
	if !p.noRedirect {
		defer logging.SetOutWriter(os.Stdout)
		logging.SetOutWriter(writer)
	}

	return fn()
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) receive(t libchan.Transport) error {
	defer logging.Logger.Debug("server: receive leaved")

	logging.Logger.Debug("server: wait for receive channel")
	receiver, err := t.WaitReceiveChannel()
	if err != nil {
		return errors.Annotate(err, "wait receive channel")
	}

	logging.Logger.Debug("server: receive channel available")

	for {
		res := CommandResponse{
			ServerVersion: p.serverVersion,
		}

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
			err = errors.Annotate(err, "decode command")
			logging.Logger.Error(err)

			res.Status = "error decoding command"
			res.Error = err.Error()

			logging.Logger.Info("send error response")
			if err := cmd.SendChannel.Send(&res); err != nil {
				return errors.Annotate(err, "send")
			}

			return nil
		}

		err := p.redirectOutput(cmd.Stdout, func() error {
			if cmd.ClientVersion != p.serverVersion {
				logging.Logger.Warn("client/server version mismatch")
				logging.Logger.Warnf("server: %s client: %s", p.serverVersion, cmd.ClientVersion)
				logging.Logger.Warn("please update your server")
				logging.Logger.Warnings++
			}

			return p.OnPromiseReceived(pr, cmd.Verbose)
		})

		res.Status = "execution successfull"
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
			logging.Logger.Debug("server: receive dying event received")
			return tomb.ErrDying
		default:
		}
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
func (p *Server) run() error {
	defer logging.Logger.Debug("server: run leaved")

	for {

		select {
		case <-p.tomb.Dying():
			logging.Logger.Debug("server: run dying event received")
			return tomb.ErrDying
		default:
		}

		c, err := p.tslListener.Accept()
		if err != nil {

			if netErr, ok := err.(net.Error); ok &&
				(netErr.Timeout() || netErr.Temporary()) {
				continue
			}
			if goagain.IsErrClosing(err) {
				logging.Logger.Debug("server: run closing error received")
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
				if err != tomb.ErrDying {
					return errors.Annotate(err, "receive")
				}
			}
			return nil
		})
	}

	return nil
}
