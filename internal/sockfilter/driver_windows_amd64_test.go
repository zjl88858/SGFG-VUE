//go:build windows && amd64

package sockfilter

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

// Manual integration check, never enabled by the ordinary test suite. The
// controller and fixed child filename must be Direct in other proxy software.
// 11.3 uses the application's managed driver lifecycle and requires elevation.
func TestRealDriverScopedTraffic(t *testing.T) {
	if os.Getenv("SGFG_SOCKFILTER_DRIVER_TEST") != "1" {
		t.Skip("manual driver test")
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	var target *net.TCPListener
	var ip net.IP
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP.To4() == nil || !n.IP.IsGlobalUnicast() || n.IP.IsLoopback() || n.IP.IsLinkLocalUnicast() {
			continue
		}
		target, err = net.ListenTCP("tcp4", &net.TCPAddr{IP: n.IP.To4()})
		if err == nil {
			ip = n.IP.To4()
			break
		}
	}
	if target == nil {
		t.Skip("no usable local IPv4 interface")
	}
	defer target.Close()
	go func() {
		for {
			c, e := target.Accept()
			if e != nil {
				return
			}
			go func() { defer c.Close(); c.SetDeadline(time.Now().Add(5 * time.Second)); io.Copy(c, c) }()
		}
	}()
	u, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
	if err != nil {
		t.Fatal(err)
	}
	defer u.Close()
	go func() {
		b := make([]byte, 1024)
		for {
			n, a, e := u.ReadFromUDPAddrPort(b)
			if e != nil {
				return
			}
			u.WriteToUDPAddrPort(b[:n], a)
		}
	}()
	proxy, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	hits := make(chan string, 16)
	go func() {
		for {
			c, e := proxy.Accept()
			if e != nil {
				return
			}
			go func() {
				defer c.Close()
				c.SetDeadline(time.Now().Add(5 * time.Second))
				var g [3]byte
				if _, e := io.ReadFull(c, g[:]); e != nil {
					return
				}
				c.Write([]byte{5, 0})
				if _, e := io.ReadFull(c, g[:]); e != nil {
					return
				}
				ap, e := readSocksAddress(c)
				if e != nil {
					return
				}
				if g[1] == 3 {
					r, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
					if e != nil {
						return
					}
					defer r.Close()
					bound := r.LocalAddr().(*net.UDPAddr).AddrPort()
					bound = netip.AddrPortFrom(bound.Addr().Unmap(), bound.Port())
					c.Write(append([]byte{5, 0, 0}, socksAddress(bound)...))
					r.SetDeadline(time.Now().Add(5 * time.Second))
					b := make([]byte, 1024)
					n, from, e := r.ReadFromUDPAddrPort(b)
					if e != nil || n < 4 {
						return
					}
					reader := &packetReader{b: b[3:n]}
					dest, e := readSocksAddress(reader)
					if e != nil {
						return
					}
					hits <- "udp:" + dest.String()
					r.WriteToUDPAddrPort(b[:n], from)
					io.Copy(io.Discard, c)
					return
				}
				hits <- "tcp:" + ap.String()
				c.Write(append([]byte{5, 0, 0}, socksAddress(netip.MustParseAddrPort("127.0.0.1:1"))...))
				io.Copy(c, c)
			}()
		}
	}()
	dir := t.TempDir()
	source := filepath.Join(dir, "probe.go")
	name := "sgfg-nf-probe.exe"
	exe := filepath.Join(dir, name)
	code := `package main
import("net";"os";"time";"io";"fmt")
func main(){
if os.Args[1]=="udp6"{c,e:=net.ListenUDP("udp6",&net.UDPAddr{});if e!=nil{panic(e)};defer c.Close();c.SetDeadline(time.Now().Add(3*time.Second));a,e:=net.ResolveUDPAddr("udp6",os.Args[2]);if e!=nil{panic(e)};_,e=c.WriteToUDP([]byte("probe"),a);if e!=nil{panic(e)};b:=make([]byte,100);n,_,e:=c.ReadFromUDP(b);fmt.Printf("reply=%s err=%v\n",b[:n],e);if e!=nil{os.Exit(1)};return}
c,e:=net.DialTimeout(os.Args[1],os.Args[2],3*time.Second);if e!=nil{panic(e)};defer c.Close();c.SetDeadline(time.Now().Add(3*time.Second));c.Write([]byte("probe"));b:=make([]byte,5);_,e=io.ReadFull(c,b);fmt.Printf("reply=%s err=%v\n",b,e);if e!=nil{os.Exit(1)}}`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", exe, source).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	for pass := 0; pass < 2; pass++ {
		s, err := Start(proxy.Addr().(*net.TCPAddr).Port, []string{name}, func(level, msg string) { t.Log(level, msg) })
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if err := s.Close(); err != nil {
					t.Error(err)
				}
			}()
			run := func(network, addr string, wantProxy bool) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				out, err := exec.CommandContext(ctx, exe, network, addr).CombinedOutput()
				if err != nil {
					t.Fatalf("%s: %v", out, err)
				}
				select {
				case hit := <-hits:
					if !wantProxy || hit != network[:3]+":"+addr {
						t.Fatalf("unexpected proxy hit %s", hit)
					}
				case <-time.After(200 * time.Millisecond):
					if wantProxy {
						t.Fatal("no SOCKS hit")
					}
				}
			}
			run("tcp4", target.Addr().String(), true)
			run("udp4", u.LocalAddr().String(), true)
			if os.Getenv("SGFG_SOCKFILTER_TEST_NONLOCAL") == "1" {
				run("tcp6", "[2001:db8::1234]:80", true)
				run("udp6", "[2001:db8::1234]:80", true)
			}
			rules := makeRules([]string{"nf-probe.exe"}, uint32(os.Getpid()))
			status := s.api.call("nf_setRulesEx", uintptr(unsafe.Pointer(&rules[0])), uintptr(len(rules)))
			runtime.KeepAlive(rules)
			if int32(status) != 0 {
				t.Fatal("cannot set scoped bypass rules")
			}
			run("tcp4", target.Addr().String(), false)
			run("udp4", u.LocalAddr().String(), false)
			// Rule buffer remains a packed C array, including its protocol field.
			if binary.LittleEndian.Uint32(rules[1][:4]) != 6 {
				t.Fatal("invalid rule")
			}
		}()
	}
}
