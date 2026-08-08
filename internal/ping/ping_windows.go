package ping

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

var (
	iphlpapi          = syscall.NewLazyDLL("iphlpapi.dll")
	procIcmpCreateFile = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpSendEcho   = iphlpapi.NewProc("IcmpSendEcho")
	procIcmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       uint32
}

const ipStatusSuccess = 0

func Ping(host string, timeoutMs int) (int, error) {
	addr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", host, err)
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return 0, fmt.Errorf("%s has no IPv4 address", host)
	}

	h, _, err := procIcmpCreateFile.Call()
	if h == 0 {
		return 0, fmt.Errorf("IcmpCreateFile: %w", err)
	}
	defer procIcmpCloseHandle.Call(h)

	payload := []byte("sgfg-ng-ping-payload-0123456789")
	replySize := unsafe.Sizeof(icmpEchoReply{}) + uintptr(len(payload)) + 8
	replyBuf := make([]byte, replySize)

	dest := uint32(ip4[0]) | uint32(ip4[1])<<8 | uint32(ip4[2])<<16 | uint32(ip4[3])<<24

	ret, _, callErr := procIcmpSendEcho.Call(
		h,
		uintptr(dest),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0,
		uintptr(unsafe.Pointer(&replyBuf[0])),
		replySize,
		uintptr(timeoutMs),
	)
	if ret == 0 {
		return 0, fmt.Errorf("IcmpSendEcho: %w", callErr)
	}

	reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if reply.Status != ipStatusSuccess {
		return 0, errors.New("destination unreachable")
	}
	return int(reply.RoundTripTime), nil
}
