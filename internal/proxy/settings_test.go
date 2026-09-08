package proxy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"sgfg-ng/internal/core"
)

func persistentTestManager(t *testing.T, path string) *Manager {
	t.Helper()
	m := NewManager()
	m.passwordKey = testPasswordKey
	m.settingsErr = m.loadSettings(path)
	return m
}

func TestSettingsRoundTripOnlyPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "settings.json")
	m := persistentTestManager(t, path)
	if m.SettingsError() != "" || !m.Options().SysProxy || m.RelayOptions().Port != 8388 {
		t.Fatal("missing-file defaults")
	}
	opts := ConnectOptions{AllowLAN: true, LocalPort: 8800, ProcessProxy: true, ProcessNames: []string{" CHROME.EXE ", "chrome.exe"}}
	if err := m.SetOptions(opts); err != nil {
		t.Fatal(err)
	}
	relay := RelayOptions{Port: 8389, Password: "test-密码\"\\"}
	if err := m.SetRelayOptions(relay); err != nil {
		t.Fatal(err)
	}
	m.sel = &core.Selection{}
	m.state = StateConnected
	m.relayOn = true
	m.traffic = &core.Traffic{}
	loaded := persistentTestManager(t, path)
	if loaded.SettingsError() != "" {
		t.Fatal(loaded.SettingsError())
	}
	if !reflect.DeepEqual(m.Options(), loaded.Options()) || loaded.RelayOptions() != relay {
		t.Fatal("settings not restored")
	}
	if loaded.sel != nil || loaded.state != StateIdle || loaded.relayOn || loaded.traffic != nil {
		t.Fatal("runtime state persisted")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["proxy"] == nil || fields["relay"] == nil || fields["version"] == nil {
		t.Fatal("unexpected persisted fields")
	}
	plain, _ := json.Marshal(relay.Password)
	if bytes.Contains(data, plain) || bytes.Contains(data, []byte(`"password":`)) {
		t.Fatal("plaintext password persisted")
	}
	// Updating proxy options and relay credentials must merge under one lock.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := loaded.SetOptions(ConnectOptions{SysProxy: true, LocalPort: 9000}); err != nil {
			t.Error(err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := loaded.SetRelayOptions(RelayOptions{Port: 9999, Password: "new"}); err != nil {
			t.Error(err)
		}
	}()
	wg.Wait()
	again := persistentTestManager(t, path)
	if again.Options().LocalPort != 9000 || !again.Options().SysProxy || again.RelayOptions().Port != 9999 {
		t.Fatal("concurrent save lost settings")
	}
}

func TestSettingsFailuresPreserveFileAndMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	broken := []byte(`{"relay":{"password":"private"},`)
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	m := persistentTestManager(t, path)
	if m.SettingsError() == "" {
		t.Fatal("corrupt configuration accepted")
	}
	if err := m.SetOptions(ConnectOptions{}); err == nil {
		t.Fatal("corrupt file silently overwritten")
	}
	if data, _ := os.ReadFile(path); !reflect.DeepEqual(data, broken) {
		t.Fatal("corrupt file changed")
	}
	// A regular file used as the parent directory reliably fails on Windows too.
	m = NewManager()
	m.passwordKey = testPasswordKey
	m.settingsPath = filepath.Join(path, "settings.json")
	before, relayBefore := m.Options(), m.RelayOptions()
	if err := m.SetOptions(ConnectOptions{AllowLAN: true}); err == nil {
		t.Fatal("write error lost")
	}
	if err := m.SetRelayOptions(RelayOptions{Port: 1234, Password: "new"}); err == nil {
		t.Fatal("relay write error lost")
	}
	if !reflect.DeepEqual(before, m.Options()) || relayBefore != m.RelayOptions() {
		t.Fatal("failed save changed memory")
	}
}

func TestSettingsValidationAndRelayWhileConnected(t *testing.T) {
	m := persistentTestManager(t, filepath.Join(t.TempDir(), "settings.json"))
	if err := m.SetRelayOptions(RelayOptions{Port: 65536}); err == nil {
		t.Fatal("invalid port accepted")
	}
	m.state = StateConnected
	if err := m.SetRelayOptions(RelayOptions{Port: 8388, Password: "saved"}); err != nil {
		t.Fatal(err)
	}
	m.relayOn = true
	if err := m.SetRelayOptions(RelayOptions{Port: 9000}); err == nil {
		t.Fatal("active relay changed")
	}
}
