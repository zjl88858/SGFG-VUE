package proxy

import "testing"

func TestProcessProxyOptions(t *testing.T) {
	m := NewManager()
	if err := m.SetOptions(ConnectOptions{ProcessProxy: true}); err == nil {
		t.Fatal("empty allowlist accepted")
	}
	if err := m.SetOptions(ConnectOptions{ProcessProxy: true, SysProxy: true, ProcessNames: []string{" CHROME.EXE ", "chrome.exe"}}); err != nil {
		t.Fatal(err)
	}
	opts := m.Options()
	if opts.SysProxy || !opts.UseSocks5 || !opts.ProcessProxy || len(opts.ProcessNames) != 1 || opts.ProcessNames[0] != "chrome.exe" {
		t.Fatalf("bad normalization: %+v", opts)
	}
	opts.ProcessNames[0] = "changed.exe"
	if m.Options().ProcessNames[0] != "chrome.exe" {
		t.Fatal("Options exposed mutable slice")
	}
	if m.Status().ProcessProxy {
		t.Fatal("status must reflect active driver, not configured switch")
	}
	m.state = StateConnected
	if err := m.SetOptions(ConnectOptions{}); err == nil {
		t.Fatal("connected options changed")
	}
	if !m.Options().ProcessProxy {
		t.Fatal("failed SetOptions mutated state")
	}
}

func TestNormalProxyDoesNotRequireSDK(t *testing.T) {
	m := NewManager()
	if err := m.SetOptions(ConnectOptions{SysProxy: true}); err != nil {
		t.Fatal(err)
	}
	if o := m.Options(); !o.SysProxy || o.ProcessProxy || o.UseSocks5 {
		t.Fatalf("normal options changed: %+v", o)
	}
	m.Shutdown() // No DLL is loaded by construction/options/shutdown.
}
