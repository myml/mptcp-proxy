package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/golog"
	"github.com/getlantern/multipath"
	"github.com/xtaci/smux"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
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
	golog.SetOutputs(os.Stderr, ioutil.Discard)
	preDialPool := make(chan Dial)
	go func() {
		for {
			ctx, _ := context.WithCancel(context.Background())
			var ds []multipath.Dialer
			for i := range paths {
				ds = append(ds, newOutboundDialer(paths[i], fmt.Sprintf("no.%d", i)))
			}
			conn, err := multipath.NewDialer("mptcp", ds).DialContext(ctx)
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second)
				continue
			}
			session, err := smux.Client(conn, nil)
			if err != nil {
				log.Fatal(err)
			}
			for {
				stream, err := session.OpenStream()
				if err != nil {
					log.Println(err)
					conn.Close()
					break
				}
				log.Println("new stream")
				preDialPool <- Dial{cancel: func() {}, conn: stream}
			}
		}
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("new conn", conn.RemoteAddr())
		go func() {
			dial := <-preDialPool
			biCopy(conn.RemoteAddr().String(), conn, dial.conn)
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
		session, err := smux.Server(conn, nil)
		if err != nil {
			log.Fatal(err)
		}
		for {
			stream, err := session.AcceptStream()
			if err != nil {
				log.Println(err)
				conn.Close()
				break
			}
			log.Println("new conn", conn.RemoteAddr())
			go func() {
				remote := <-preConnPool
				biCopy(conn.RemoteAddr().String(), stream, remote)
			}()
		}
	}
}

func biCopy(name string, a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, err := io.Copy(a, b)
		if err != nil {
			log.Println(err)
		}
		wg.Done()
		a.Close()
	}()
	go func() {
		_, err := io.Copy(b, a)
		if err != nil {
			log.Println(err)
		}
		wg.Done()
		b.Close()
	}()
	wg.Wait()
	log.Println(name, "close")
}
