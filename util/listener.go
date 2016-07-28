package util

import (
	"net"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
type TimeoutListener struct {
	*net.TCPListener
}

////////////////////////////////////////////////////////////////////////////////
func (ln *TimeoutListener) Accept() (c net.Conn, err error) {
	ln.SetDeadline(time.Now().Add(1 * time.Second))

	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)

	return tc, nil
}

////////////////////////////////////////////////////////////////////////////////
func NewTimeoutListener(inner *net.TCPListener) *TimeoutListener {
	tl := TimeoutListener{inner}
	return &tl
}
