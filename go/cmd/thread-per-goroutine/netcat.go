package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"sync/atomic"
	"time"
)

var (
	serverAddress = flag.String("-l", "0.0.0.0:19999", "start server address")
	dialAddress = flag.String("-c","0.0.0.0:19999", "connect server address" )
)

var mesure uint64

func runServer() {
	go func() {
		for {
			// 每秒测量一次吞吐
			now := time.Now()
			time.Sleep(time.Second)
			elapsed := time.Since(now)
			nBytes := atomic.LoadUint64(&mesure)
			atomic.StoreUint64(&mesure, 0)
			if nBytes > 0 {
				mib :=  float64(nBytes) / (1024.0 * 1024) / elapsed.Seconds()
				log.Printf("%.3f MiB/s\n",mib);
			}
		}
	}()

	ln, err := net.Listen("tcp", *serverAddress)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		// stdin -> socket
		go func(conn net.Conn) {
			buf := make([]byte, 4096);
			for {
				nr, err := os.Stdin.Read(buf)
				if err != nil {
					// stdin closed
					if err == io.EOF {
						break
					}

					panic(err)
				}

				nw, err := conn.Write(buf[:nr])
				if err != nil {
					panic(err)
				}

				// write socket mesure
				atomic.AddUint64(&mesure, uint64(nw))

				if nw != nr {
					log.Printf("write stdin -> socket failed, write socket: %d bytes, read stdin: %d bytes",
						nw, nr)
					continue
				}
			}
		}(conn)

		// socket -> stdout
		go func(conn net.Conn) {
			buf := make([]byte, 4096)
			for {
				nr, err := conn.Read(buf)
				if err != nil {
					// client closed
					if err == io.EOF {
						break
					}
					panic(err)
				}

				// read socket mesure
				atomic.AddUint64(&mesure, uint64(nr))

				nw, err := os.Stdout.Write(buf[:nr])
				if err != nil {
					panic(err)
				}

				if nw != nr {
					log.Printf("read socket -> stdout failed, read socket: %d bytes, write stdout: %d bytes",
						nr, nw)
					continue
				}
			}
		}(conn)
	}
}

func runClient()  {
	net.Dial("tcp", *dialAddress)
}

func main() {
	atomic.StoreUint64(&mesure, 0)
	if *serverAddress != "" {
		runServer()
	} else if *dialAddress != "" {
		runClient()
	}
}