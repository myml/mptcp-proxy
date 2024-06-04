package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/buaazp/fasthttprouter"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

type MutexConn struct {
	readChan  chan []byte
	writeLock sync.Mutex
	conn      net.Conn
}

var serviceMap sync.Map

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	var listen string
	var chanlen, bufferSize int64
	flag.StringVar(&listen, "l", ":11111", "listen addr, e.g. 127.0.0.1:11111")
	flag.Int64Var(&chanlen, "chanlen", 100, "reader channel size")
	flag.Int64Var(&bufferSize, "buffsize", 1024*1024, "rader buffer size")
	flag.Parse()

	var buffPool = sync.Pool{
		New: func() any {
			return make([]byte, 1024*1024)
		},
	}

	router := fasthttprouter.New()
	router.POST("/new", func(ctx *fasthttp.RequestCtx) {
		addr := string(ctx.QueryArgs().Peek("service"))
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Println(err)
			return
		}
		id := uuid.New().String()
		mc := MutexConn{conn: conn, readChan: make(chan []byte, chanlen)}
		serviceMap.Store(id, &mc)
		ctx.WriteString(id)

		go func() {
			defer conn.Close()
			defer serviceMap.Delete(id)
			for {
				readerBuffer := buffPool.Get().([]byte)
				n, err := conn.Read(readerBuffer)
				if err != nil {
					return
				}
				mc.readChan <- readerBuffer[:n]
			}
		}()
	})
	router.GET("/r", func(ctx *fasthttp.RequestCtx) {
		id := string(ctx.QueryArgs().Peek("id"))
		v, ok := serviceMap.Load(id)
		if !ok {
			log.Println("can not found id", id)
			ctx.SetStatusCode(http.StatusNotFound)
			return
		}
		mc := v.(*MutexConn)
		data := <-mc.readChan
		_, err := ctx.Write(data)
		buffPool.Put(data)
		if err != nil {
			log.Println("write data to client", err)
			return
		}
	})
	router.POST("/w", func(ctx *fasthttp.RequestCtx) {
		id := string(ctx.QueryArgs().Peek("id"))
		v, ok := serviceMap.Load(id)
		if !ok {
			log.Println("can not found id", id)
			ctx.SetStatusCode(http.StatusNotFound)
			return
		}
		mc := v.(*MutexConn)
		mc.writeLock.Lock()
		defer mc.writeLock.Unlock()
		err := ctx.Request.BodyWriteTo(mc.conn)
		if err != nil {
			log.Println("copy data to service", err)
			return
		}
	})
	log.Println("server on", listen)
	log.Fatal(fasthttp.ListenAndServe(listen, router.Handler))
}
