package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"github.com/getlantern/multipath"
)

func main() {
	var server string
	var client string
	var paths string
	var remote string
	flag.StringVar(&server, "s", "", "server mode, listen addr. e.g. 0.0.0.0:12345")
	flag.StringVar(&client, "c", "", "client mode, listen addr. e.g. 0.0.0.0:5001")
	flag.StringVar(&paths, "p", "", "client mode, server addrs. e.g. 127.0.0.1:12345,192.168.0.2:12345")
	flag.StringVar(&remote, "r", "", "server mode, proxy to remote server. e.g. 127.0.0.1:5001")
	flag.Parse()
	if len(client) > 0 {
		runClient(client, strings.Split(paths, ","))
		return
	}
	if len(server) > 0 {
		runServer(server, remote)
		return
	}
	flag.PrintDefaults()
}

func runClient(listen string, paths []string) {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listen tcp at", listen)

	type Dial struct {
		cancel func()
		conn   net.Conn
	}
	preDialPool := make(chan Dial)
	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			var ds []multipath.Dialer
			for i := range paths {
				ds = append(ds, newOutboundDialer(paths[i], fmt.Sprintf("no.%d", i)))
			}
			remote, err := multipath.NewDialer("mptcp", ds).DialContext(ctx)
			if err != nil {
				log.Fatal(err)
			}
			preDialPool <- Dial{cancel: cancel, conn: remote}
		}
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			dial := <-preDialPool
			biCopy(conn, dial.conn)
			dial.cancel()
		}()
	}
}

func runServer(listen string, remote string) {
	listeners := make([]net.Listener, 0)
	trackers := make([]multipath.StatsTracker, 0)

	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	listeners = append(listeners, l)
	trackers = append(trackers, multipath.NullTracker{})
	ml := multipath.NewListener(listeners, trackers)

	log.Println("listen mptcp at", listen)

	preConnPool := make(chan net.Conn)
	go func() {
		for {
			remote, err := net.Dial("tcp", remote)
			if err != nil {
				log.Fatal(err)
			}
			preConnPool <- remote
		}
	}()

	for {
		conn, err := ml.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			remote := <-preConnPool
			biCopy(conn, remote)
		}()
	}
}

func biCopy(a, b net.Conn) {
	closeCh := make(chan bool, 2)
	go func() {
		_, err := io.Copy(a, b)
		if err != nil {
			log.Println(err)
		}
		closeCh <- true
	}()
	go func() {
		_, err := io.Copy(b, a)
		if err != nil {
			log.Println(err)
		}
		closeCh <- true
	}()
	<-closeCh
	a.Close()
	b.Close()
	close(closeCh)
}
