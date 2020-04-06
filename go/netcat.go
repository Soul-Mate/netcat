package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
)

var (
	serverAddress = flag.String("-l", "0.0.0.0:19999", "start server address")
	dialAddress = flag.String("-c","0.0.0.0:19999", "connect server address" )
)

func runServer() {
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
	if *serverAddress != "" {
		runServer()
	} else if *dialAddress != "" {
		runClient()
	}
}