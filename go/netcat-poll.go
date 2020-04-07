package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

var (
	serverAddress = flag.String("l", "0.0.0.0:19999", "start server address")
	// dialAddress   = flag.String("-c", "0.0.0.0:19999", "connect server address")
)

var mesure uint64

func writeAllClient(pollFds []unix.PollFd, serverFd int32, buf []byte, n int) {
	for i := 0; i < len(pollFds); i++ {
		if pollFds[i].Fd == serverFd || pollFds[i].Fd == int32(unix.Stdin) {
			continue
		}

		if pollFds[i].Revents&unix.POLLOUT == unix.POLLOUT {
			nw, err := unix.Write(int(pollFds[i].Fd), buf[:n])
			if err != nil {
				panic(err)
			}

			if nw != n {
				log.Printf("write to %d client failed, need write %d, write %d", pollFds[i].Fd, n, nw)
			}

			atomic.AddUint64(&mesure, uint64(nw))
		}
	}
}

func runServer(host string, port int) error {
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return err
	}

	ip4 := ipAddr.IP.To4()
	if len(ip4) != net.IPv4len {
		return fmt.Errorf("invalid ipv4 value")
	}

	serverFd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal(err)
	}

	sa := unix.SockaddrInet4{
		Port: port,
		Addr: [4]byte{ipAddr.IP[0], ipAddr.IP[1], ipAddr.IP[2], ipAddr.IP[3]},
	}

	if err = unix.Bind(serverFd, &sa); err != nil {
		return err
	}

	if err = unix.Listen(serverFd, 128); err != nil {
		return err
	}

	pollFds := make([]unix.PollFd, 0)

	pollFds = append(pollFds,
		unix.PollFd{
			Fd:      int32(serverFd),
			Events:  unix.POLLIN,
			Revents: -1,
		},
		unix.PollFd{
			Fd:      int32(unix.Stdin),
			Events:  unix.POLLIN,
			Revents: -1,
		})

	outBuf := make([]byte, 4096)
	inBuf := make([]byte, 4096)
	for {
		nReady, err := unix.Poll(pollFds, -1)
		if err != nil {
			panic(err)
		}

		for i := 0; i < len(pollFds); i++ {
			if pollFds[i].Fd == int32(serverFd) && pollFds[i].Revents&unix.POLLIN == 1 {
				connFd, _, err := unix.Accept(serverFd)
				if err != nil {
					panic(err)
				}

				pollFds = append(pollFds, unix.PollFd{
					Fd: int32(connFd),
					// Events: unix.POLLIN | unix.POLLOUT,
					Events: unix.POLLOUT,
				})

				fmt.Println(pollFds)

				if nReady--; nReady == 0 {
					break
				}

			} else if pollFds[i].Fd == int32(unix.Stdin) && pollFds[i].Revents&unix.POLLIN == 1 {
				nr, err := unix.Read(unix.Stdin, inBuf)
				if err != nil {
					panic(err)
				}

				writeAllClient(pollFds, int32(serverFd), inBuf, nr)

				if nReady--; nReady == 0 {
					break
				}

			} else {
				if pollFds[i].Revents&unix.POLLIN == unix.POLLIN {
					nr, err := unix.Read(int(pollFds[i].Fd), outBuf)
					if err != nil {
						panic(err)
					}

					if nr == 0 {
						log.Printf("client closed")
						continue
					}

					atomic.AddUint64(&mesure, uint64(nr))

					os.Stdout.Write(outBuf[:nr])

					if nReady--; nReady == 0 {
						break
					}
				}
			}
		}
	}

}

func main() {
	flag.Parse()
	atomic.StoreUint64(&mesure, 0)

	// mesure
	go func() {
		for {
			// 每秒测量一次吞吐
			now := time.Now()
			time.Sleep(time.Second)
			elapsed := time.Since(now)
			nBytes := atomic.LoadUint64(&mesure)
			atomic.StoreUint64(&mesure, 0)
			if nBytes > 0 {
				mib := float64(nBytes) / (1024.0 * 1024) / elapsed.Seconds()
				log.Printf("%.3f MiB/s\n", mib)
			}
		}
	}()

	if *serverAddress != "" {
		sep := strings.Split(*serverAddress, ":")
		if len(sep) != 2 {
			log.Fatal("invalid address")
		}

		port, err := strconv.Atoi(sep[1])
		if err != nil {
			log.Fatalf("strconv.Atoid port failed: %s", err.Error())
		}

		log.Println(runServer(sep[0], port))
	}

}
