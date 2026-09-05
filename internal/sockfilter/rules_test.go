package sockfilter

import (
	"encoding/binary"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestNormalizeNames(t *testing.T) {
	names, err := NormalizeNames([]string{" Chrome.EXE ", "", "chrome.exe", "测试.exe", "curl.exe"})
	if err != nil || !reflect.DeepEqual(names, []string{"chrome.exe", "测试.exe", "curl.exe"}) {
		t.Fatalf("%v %v", names, err)
	}
	for _, invalid := range []string{"*", "*.exe", "chrome", "exe", ".exe", `C:\chrome.exe`, "../chrome.exe", "bad\x00.exe", "bad?.exe", strings.Repeat("a", 251) + ".exe"} {
		if _, err := NormalizeNames([]string{invalid}); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
	names, err = NormalizeNames(nil)
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("empty list: %v, %v", names, err)
	}
}

func TestRulesPackedABIAndDefaultBypass(t *testing.T) {
	rules := makeRules([]string{"chrome.exe", "测试.exe"}, 1234)
	if len(rules) != 6 || len(rules[0]) != 643 {
		t.Fatal("bad rule count/size")
	}
	if binary.LittleEndian.Uint32(rules[0][4:8]) != 1234 || binary.LittleEndian.Uint32(rules[0][79:83]) != 0 {
		t.Fatal("self must bypass first")
	}
	if rules[len(rules)-1] != (rule{}) {
		t.Fatal("default must bypass")
	}
	for i, name := range []string{"chrome.exe", "测试.exe"} {
		for j, protocol := range []uint32{6, 17} {
			r := rules[1+2*i+j]
			if binary.LittleEndian.Uint32(r[:4]) != protocol {
				t.Fatal("bad protocol")
			}
			flag, direction := uint32(16), byte(2)
			if protocol == 17 {
				flag, direction = 2, 0
			}
			if binary.LittleEndian.Uint32(r[79:83]) != flag || r[8] != direction {
				t.Fatal("bad flags/direction")
			}
			var chars []uint16
			for off := 83; off < 603; off += 2 {
				ch := binary.LittleEndian.Uint16(r[off:])
				if ch == 0 {
					break
				}
				chars = append(chars, ch)
			}
			if string(utf16.Decode(chars)) != `*\`+name {
				t.Fatalf("missing filename boundary: %v", chars)
			}
		}
	}
}

func TestSockaddrRoundTrip(t *testing.T) {
	for _, value := range []string{"1.2.3.4:443", "[2001:db8::1234]:8080", "[::ffff:192.0.2.1]:53"} {
		want := netip.MustParseAddrPort(value)
		got, err := fromAddrPort(want).addrPort()
		if err != nil || got != want {
			t.Fatalf("%s -> %s, %v", want, got, err)
		}
	}
	if _, err := (address{}).addrPort(); err == nil {
		t.Fatal("invalid family accepted")
	}
}

func TestBypass(t *testing.T) {
	for _, value := range []string{"127.0.0.1:80", "127.10.1.2:80", "[::1]:80", "[::ffff:127.0.0.1]:80", "[fe80::1]:80", "169.254.1.2:80", "224.0.0.1:53", "255.255.255.255:53", "[ff02::1]:53"} {
		if !bypass(netip.MustParseAddrPort(value)) {
			t.Errorf("must bypass %s", value)
		}
	}
	for _, value := range []string{"1.1.1.1:443", "192.168.1.1:80", "[2001:db8::1]:443"} {
		if bypass(netip.MustParseAddrPort(value)) {
			t.Errorf("must proxy %s", value)
		}
	}
}
