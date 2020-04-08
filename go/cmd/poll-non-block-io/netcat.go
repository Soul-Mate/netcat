package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Soul-Mate/netcat/pkg/util"
	"golang.org/x/sys/unix"
)

var (
	mesure        *util.Mesure
	serverAddress = flag.String("l", "", "start server address")
	dialAddress   = flag.String("c", "", "connect server address")
	chargen       = flag.Bool("chargen", false, "closed server read client, client may block in write")
)

func unregisterPollFd(pollFds *[]unix.PollFd, i int) {
	if i+1 < len(*pollFds) {
		*pollFds = append((*pollFds)[:i], (*pollFds)[i+1:]...)
	} else {
		*pollFds = (*pollFds)[:i]
	}
}

func writeAllClient(pollFds *[]unix.PollFd, serverFd int32, buf []byte, n int) {
	for i := 0; i < len(*pollFds); i++ {
		if (*pollFds)[i].Fd == serverFd || (*pollFds)[i].Fd == int32(unix.Stdin) {
			continue
		}

		if (*pollFds)[i].Revents&unix.POLLOUT == unix.POLLOUT {
			log.Println((*pollFds)[i].Revents&unix.POLLOUT == unix.POLLOUT)
			nw, err := unix.Write(int((*pollFds)[i].Fd), buf[:n])
			if err != nil {
				// client closed
				if err == unix.EPIPE {
					log.Printf("write to %d cliend error: %s", (*pollFds)[i].Fd, err.Error())
					// unregister
					unregisterPollFd(pollFds, i)
					fmt.Println(*pollFds)
					continue
				}

				// operation would block
				if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
					// register pollout
					// unregister pollin
					log.Printf("write to %d client wolud blocked", (*pollFds)[i].Fd)
					continue
				}
			}

			if nw != n {
				log.Printf("write to %d client failed, need write %d, write %d", (*pollFds)[i].Fd, n, nw)
			}

			mesure.Add(uint64(nw))
		}
	}
}

func runServer(ipv4 [4]byte, port int) error {
	serverFd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal(err)
	}
	sa := unix.SockaddrInet4{
		Port: port,
		Addr: ipv4,
	}

	if err = unix.Bind(serverFd, &sa); err != nil {
		return err
	}

	if err = unix.Listen(serverFd, 128); err != nil {
		return err
	}

	pollFds := make([]unix.PollFd, 0, 1024)
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

	nr, nw := 0, 0
	outBuf := make([]byte, 4096)
	inToSocketBuf := make([]byte, 4096)
	// todo test
	go func() {
		time.Sleep(time.Second * 20)
		*chargen = false
		fmt.Println(*chargen)
	}()
	for {
		nReady, err := unix.Poll(pollFds, 1000) // 500ms
		if err != nil {
			panic(err)
		}
		// accept new connection
		if pollFds[0].Revents&unix.POLLIN == unix.POLLIN {
			connFd, _, err := unix.Accept(serverFd)
			if err != nil {
				panic(err)
			}

			pollFds = append(pollFds, unix.PollFd{
				Fd:     int32(connFd),
				Events: unix.POLLIN | unix.POLLOUT,
			})

			// set non blocking
			if _, err = util.SetNonBlocking(connFd); err != nil {
				log.Println(err)
				syscall.Close(connFd)
				continue
			}

			if nReady--; nReady <= 0 {
				continue
			}
		}

		if pollFds[1].Revents&unix.POLLIN == unix.POLLIN {
			if nr, err = os.Stdin.Read(inToSocketBuf); err != nil {
				panic(err)
			}

			if nReady--; nReady <= 0 {
				continue
			}
		}

		for i := 2; i < len(pollFds); i++ {
			if *chargen == false && pollFds[i].Revents&unix.POLLIN == unix.POLLIN {
				nr, err := unix.Read(int(pollFds[i].Fd), outBuf)
				if err != nil {
					log.Printf("read %d client error: %s", pollFds[i].Fd, err.Error())
					unregisterPollFd(&pollFds, i)
					continue
				}

				if nr == 0 {
					log.Printf("client %d closed", pollFds[i].Fd)
					unregisterPollFd(&pollFds, i)
					continue
				}

				mesure.Add(uint64(nr))

				os.Stdout.Write(outBuf[:nr])
			}

			if pollFds[i].Revents&unix.POLLOUT == unix.POLLOUT {
				if nr >= 0 {
					if nw, err = unix.Write(int(pollFds[i].Fd), inToSocketBuf[:nr]); err != nil {
						// client closed
						if err == unix.EPIPE {
							log.Printf("write to %d cliend error: %s", pollFds[i].Fd, err.Error())
							// unregister
							unregisterPollFd(&pollFds, i)
							continue
						}

						// operation would block
						if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
							// register pollout
							// unregister pollin
							log.Printf("write to %d client wolud blocked", pollFds[i].Fd)
							continue
						}
					}

					mesure.Add(uint64(nw))

					if nw != nr {
						log.Printf("write to %d client failed, need write %d, write %d", pollFds[i].Fd, nr, nw)
					}
				}
			}
		}
	}

	return nil
}

func runClient(host string, port int) error {
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return err
	}

	ip4 := ipAddr.IP.To4()
	if len(ip4) != net.IPv4len {
		return fmt.Errorf("only ipv4 address")
	}

	serverFd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal(err)
	}

	sa := unix.SockaddrInet4{
		Port: port,
		Addr: [4]byte{ipAddr.IP[0], ipAddr.IP[1], ipAddr.IP[2], ipAddr.IP[3]},
	}

	if err = unix.Connect(serverFd, &sa); err != nil {
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

	// set non blocking io
	if _, err = util.SetNonBlocking(serverFd); err != nil {
		return err
	}

	inToSocketBuf := make([]byte, 4096)
	socketToInBuf := make([]byte, 4096)
	writeBuffer := make([]byte, 0, 0)

	for {
		nReady, err := unix.Poll(pollFds, 1000)
		if err != nil {
			panic(err)
		}

		fmt.Println(nReady)
		// server is read
		if pollFds[0].Revents&unix.POLLIN == unix.POLLIN {
			nr, err := unix.Read(serverFd, socketToInBuf)
			if err != nil {
				log.Printf("read server error: %s", err.Error())
				continue
			}

			if nr == 0 {
				unix.Close(serverFd)
				log.Fatalf("read but server closed")
			}

			os.Stdout.Write(socketToInBuf[:nr])

			// if nReady--; nReady <= 0 {
			// 	continue
			// }
		}

		// write to server
		if pollFds[1].Revents&unix.POLLIN == unix.POLLIN {
			nr, err := unix.Read(unix.Stdin, inToSocketBuf)
			if err != nil {
				log.Printf("read server error: %s", err.Error())
				continue
			}

			if nr > 0 {
				nw, err := unix.Write(serverFd, inToSocketBuf[:nr])
				if err != nil && err == unix.EPIPE {
					unix.Close(serverFd)
					log.Fatalf("write buf server closed")
				}

				if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
					if nw > 0 {
						// write to buffer, the buffer is unlimited
						if cap(writeBuffer) > nw {
							copy(writeBuffer, inToSocketBuf[:nw])
							fmt.Printf("copy %d byte to writeBuffer cap: %d\n", nw, cap(writeBuffer))
						} else {
							writeBuffer = append(writeBuffer, inToSocketBuf[:nw]...)
							fmt.Printf("append %d byte to writeBuffer cap: %d\n", nw, cap(writeBuffer))
						}
					} else {
						// write to buffer, the buffer is unlimited
						writeBuffer = append(writeBuffer, inToSocketBuf[:nr]...)
						fmt.Printf("append %d byte to writeBuffer cap: %d\n", nr, cap(writeBuffer))
					}

					// unregister stdin pollin
					pollFds[1].Events &= ^unix.POLLIN
					// register server socket pollout
					pollFds[0].Events |= unix.POLLOUT
					fmt.Println("registe poll out: ", pollFds[0].Events&unix.POLLOUT == unix.POLLOUT)
				}
			}
		}

		if pollFds[0].Revents&unix.POLLOUT == unix.POLLOUT {
			fmt.Println("server is pollout")
		}
	}

	return nil
}

func main() {
	flag.Parse()
	mesure = util.NewMesure()
	go mesure.Run(0)

	if *serverAddress != "" {
		ipv4Byte, port, err := util.ResloveEndpoint(*serverAddress)
		if err != nil {
			log.Fatal(err)
		}

		log.Println(runServer(ipv4Byte, port))
	} else if *dialAddress != "" {
		sep := strings.Split(*dialAddress, ":")
		if len(sep) != 2 {
			log.Fatal("invalid address")
		}

		port, err := strconv.Atoi(sep[1])
		if err != nil {
			log.Fatalf("strconv.Atoid port failed: %s", err.Error())
		}
		runClient(sep[0], port)
	}

}
