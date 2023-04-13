package main

import (
	"context"
	"fmt"
	"log"
	"net"
)

type targetedDailer struct {
	localDialer net.Dialer
	remoteAddr  string
	label       string
}

func newOutboundDialer(inputRemoteAddr string, label string) *targetedDailer {
	td := &targetedDailer{
		localDialer: net.Dialer{},
		remoteAddr:  inputRemoteAddr,
	}
	return td
}

func (td *targetedDailer) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := td.localDialer.DialContext(ctx, "tcp", td.remoteAddr)
	if err != nil {
		return nil, err
	}
	log.Printf("Dialed to %v->%v", conn.LocalAddr(), td.remoteAddr)
	return conn, err
}

func (td *targetedDailer) Label() string {
	return fmt.Sprintf("%s|%s", td.label, td.remoteAddr)
}
