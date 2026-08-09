//go:build windows

package ping

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

var (
	iphlpapi           = syscall.NewLazyDLL("iphlpapi.dll")
	icmpCreateFileProc = iphlpapi.NewProc("IcmpCreateFile")
	icmpSendEchoProc   = iphlpapi.NewProc("IcmpSendEcho")
	icmpCloseProc      = iphlpapi.NewProc("IcmpCloseHandle")
)

type echoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       uint32
}

func Ping(host string, timeoutMs int) (int, error) {
	addr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", host, err)
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return 0, fmt.Errorf("%s has no IPv4 address", host)
	}
	handle, _, callErr := icmpCreateFileProc.Call()
	if handle == 0 {
		return 0, fmt.Errorf("IcmpCreateFile: %w", callErr)
	}
	defer icmpCloseProc.Call(handle)

	payload := []byte("sgfg-tui-ping")
	replySize := unsafe.Sizeof(echoReply{}) + uintptr(len(payload)) + 8
	replyBuffer := make([]byte, replySize)
	destination := uint32(ip4[0]) | uint32(ip4[1])<<8 | uint32(ip4[2])<<16 | uint32(ip4[3])<<24
	result, _, callErr := icmpSendEchoProc.Call(
		handle,
		uintptr(destination),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0,
		uintptr(unsafe.Pointer(&replyBuffer[0])),
		replySize,
		uintptr(timeoutMs),
	)
	if result == 0 {
		return 0, fmt.Errorf("IcmpSendEcho: %w", callErr)
	}
	reply := (*echoReply)(unsafe.Pointer(&replyBuffer[0]))
	if reply.Status != 0 {
		return 0, errors.New("destination unreachable")
	}
	return int(reply.RoundTripTime), nil
}
