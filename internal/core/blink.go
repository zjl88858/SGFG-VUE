package core

import (
	"net"

	"github.com/zjl88858/oh-my-hysteria/extras/v2/obfs"
)

// All Blink knobs are passed through, including explicit zero padding/junk.
type BlinkConfig struct {
	Password               string
	MaxPacketSize          int
	PaddingMax             *int
	JunkPackets            *int
	Fragmentation          bool
	MaxSessions            int
	MaxReassemblyBytes     int
	VerifyPacketsPerSecond int
}

func (b BlinkConfig) options() obfs.BlinkOptions {
	return obfs.BlinkOptions{Password: []byte(b.Password), MaxPacketSize: b.MaxPacketSize,
		PaddingMax: b.PaddingMax, JunkPackets: b.JunkPackets, Fragmentation: b.Fragmentation,
		MaxSessions: b.MaxSessions, MaxReassemblyBytes: b.MaxReassemblyBytes, VerifyPacketsPerSecond: b.VerifyPacketsPerSecond}
}

// Validate through upstream without opening a socket, resolving a hostname or
// including any deployment-specific value in an error message.
func (b BlinkConfig) validate() error {
	wrapped, err := obfs.WrapPacketConnBlink(validationPacketConn{}, b.options())
	if err != nil {
		return err
	}
	return wrapped.Close()
}

type validationPacketConn struct{ net.PacketConn }

func (validationPacketConn) Close() error { return nil }
