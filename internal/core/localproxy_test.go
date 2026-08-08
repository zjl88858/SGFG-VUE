package core

import "testing"

func TestParseUDPRelayHeaderTruncated(t *testing.T) {
	cases := [][]byte{
		{0x00, 0x00, 0x00, 0x03},
		{0x00, 0x00, 0x00, 0x01},
		{0x00, 0x00, 0x00, 0x04},
		{0x00, 0x00, 0x00, 0x03, 0x05, 'a', 'b'},
		{0x00, 0x00, 0x00, 0x01, 1, 2, 3, 4},
	}
	for i, c := range cases {
		if _, _, err := parseUDPRelayHeader(c); err == nil {
			t.Errorf("case %d: expected error for truncated packet", i)
		}
	}
}

func TestParseUDPRelayHeaderValid(t *testing.T) {
	addr, hdrLen, err := parseUDPRelayHeader([]byte{0x00, 0x00, 0x00, 0x03, 0x0b, 'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', 0x00, 0x50, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	if addr != "example.com:80" {
		t.Errorf("got %s", addr)
	}
	if hdrLen != 18 {
		t.Errorf("hdrLen=%d", hdrLen)
	}
}
