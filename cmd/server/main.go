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
	readLock     sync.Mutex
	writeLock    sync.Mutex
	conn         net.Conn
	readerBuffer [1024 * 1024]byte
}

var serviceMap sync.Map

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	var listen string
	flag.StringVar(&listen, "l", ":11111", "listen addr, e.g. 127.0.0.1:11111")
	flag.Parse()

	router := fasthttprouter.New()
	router.POST("/new", func(ctx *fasthttp.RequestCtx) {
		addr := string(ctx.QueryArgs().Peek("service"))
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Println(err)
			return
		}
		id := uuid.New().String()
		mc := MutexConn{conn: conn}
		serviceMap.Store(id, &mc)
		ctx.WriteString(id)
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
		mc.readLock.Lock()
		defer mc.readLock.Unlock()
		n, err := mc.conn.Read(mc.readerBuffer[:])
		if err != nil {
			log.Println("read data from service", err)
			ctx.SetStatusCode(http.StatusInternalServerError)
			return
		}
		_, err = ctx.Write(mc.readerBuffer[:n])
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
