package proxy

import (
	"strings"
	"testing"
)

type fakeSystemProxy struct {
	available bool
	reason    string
}

func (f *fakeSystemProxy) Available() (bool, string) { return f.available, f.reason }
func (*fakeSystemProxy) Enable(string) error         { return nil }
func (*fakeSystemProxy) Restore() error              { return nil }

func TestManagerDefaultsFollowPlatformCapability(t *testing.T) {
	manager := NewManager(&fakeSystemProxy{available: false, reason: "unsupported"})
	if manager.Options().SysProxy {
		t.Fatal("system proxy should default off when unavailable")
	}
	manager = NewManager(&fakeSystemProxy{available: true})
	if !manager.Options().SysProxy {
		t.Fatal("system proxy should default on when available")
	}
}

func TestSetOptionsValidatesAndEnforcesExclusivity(t *testing.T) {
	manager := NewManager(&fakeSystemProxy{available: true})
	if err := manager.SetOptions(ConnectOptions{LocalPort: 80}); err == nil {
		t.Fatal("expected invalid port error")
	}
	if err := manager.SetOptions(ConnectOptions{UseSocks5: true, SysProxy: true, LocalPort: 7788}); err != nil {
		t.Fatal(err)
	}
	options := manager.Options()
	if options.SysProxy || !options.UseSocks5 {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestSetOptionsRejectsUnavailableSystemProxy(t *testing.T) {
	manager := NewManager(&fakeSystemProxy{reason: "manual only"})
	err := manager.SetOptions(ConnectOptions{SysProxy: true})
	if err == nil || !strings.Contains(err.Error(), "manual only") {
		t.Fatalf("unexpected error: %v", err)
	}
}
