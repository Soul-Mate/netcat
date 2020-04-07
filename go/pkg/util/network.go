package util

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func ResloveEndpoint(address string) (ipv4 [4]byte, port int, err error) {
	sep := strings.Split(address, ":")
	if len(sep) != 2 {
		err = fmt.Errorf("invalid endpoint")
		return
	}
	if port, err = strconv.Atoi(sep[1]); err != nil {
		err = fmt.Errorf("strconv.Atoi port error: %s", err.Error())
		return
	}

	// 0.0.0.0 port
	if sep[0] == "" {
		ipv4[0] = '0'
		ipv4[1] = '0'
		ipv4[2] = '0'
		ipv4[3] = '0'
		return
	}

	var ipAddr *net.IPAddr
	if ipAddr, err = net.ResolveIPAddr("ip", sep[0]); err != nil {
		return
	}

	ipAddr.IP = ipAddr.IP.To4()
	if len(ipAddr.IP) != net.IPv4len {
		err = fmt.Errorf("only support ipv4 address")
		return
	}

	ipv4[0] = ipAddr.IP[0]
	ipv4[1] = ipAddr.IP[1]
	ipv4[2] = ipAddr.IP[2]
	ipv4[3] = ipAddr.IP[3]
	return
}

func SetNonBlocking(fd int) (old int, err error) {
	if old, err = unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0); err != nil {
		return
	}

	newOp := old | unix.O_NONBLOCK
	_, err = unix.FcntlInt(uintptr(fd), unix.F_SETFL, newOp)
	return
}
