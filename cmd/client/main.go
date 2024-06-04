package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	var server, listen, service string
	flag.StringVar(&listen, "l", ":12345", "listen addr, e.g. 127.0.0.1:12345")
	flag.StringVar(&server, "s", "", "server list, e.g. server1:12345,server2:12345")
	flag.StringVar(&service, "r", "", "service addr, e.g. service:1231")
	flag.Parse()
	if len(server) == 0 || len(service) == 0 {
		flag.PrintDefaults()
		return
	}
	servers := strings.Split(server, ",")
	for i := range servers {
		if !strings.HasPrefix(servers[i], "http://") {
			servers[i] = "http://" + servers[i]
		}
		log.Println(listen, "=>", servers[i], "=>", service)
	}
	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("new client")
		go clientHandle(conn, servers, service)
	}
}

func clientHandle(conn net.Conn, servers []string, service string) {
	defer conn.Close()

	resp, err := http.Post(servers[0]+"/new?service="+service, "", nil)
	if err != nil {
		log.Println("connect to server", err)
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("read id: %w", err)
		return
	}
	id := string(data)
	log.Println("get id", id)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		var readerBuffer [1024 * 1024]byte
		for {
			n, err := conn.Read(readerBuffer[:])
			if err != nil {
				log.Println("read data from server", err)
				return
			}
			server := servers[rand.Intn(len(servers))]
			resp, err := http.Post(server+"/w?id="+id, "", bytes.NewReader(readerBuffer[:n]))
			if err != nil {
				log.Println("send data to server", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
		}
	}()
	for i := range servers {
		server := servers[i]
		go func() {
			for {
				resp, err := http.Get(server + "/r?id=" + id)
				if err != nil {
					log.Println("send data to server", err)
					return
				}
				if resp.StatusCode != http.StatusOK {
					return
				}
				_, err = io.Copy(conn, resp.Body)
				if err != nil {
					log.Println("read data from server", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
