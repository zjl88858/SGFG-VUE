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
	for i, tc := range cases {
		if _, _, err := parseUDPRelayHeader(tc); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestParseUDPRelayHeaderValid(t *testing.T) {
	addr, headerLen, err := parseUDPRelayHeader([]byte{0, 0, 0, 3, 11, 'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm', 0, 80, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	if addr != "example.com:80" || headerLen != 18 {
		t.Fatalf("got addr=%s headerLen=%d", addr, headerLen)
	}
}

func TestBuildUDPRelayHeaderRejectsLongDomain(t *testing.T) {
	host := make([]byte, 256)
	for i := range host {
		host[i] = 'a'
	}
	if got := buildUDPRelayHeader(string(host) + ":53"); got != nil {
		t.Fatalf("expected nil, got %x", got)
	}
}
