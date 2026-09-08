package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPasswordKey() ([]byte, error) { return derivePasswordKey("test-machine", "test-user"), nil }

func TestPasswordEncryptionBindingAndTampering(t *testing.T) {
	key, _ := testPasswordKey()
	password := "测试-password-\"\\"
	first, err := encryptPassword(password, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := encryptPassword(password, key)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("nonce reused")
	}
	if plain, err := decryptPassword(first, key); err != nil || plain != password {
		t.Fatal("round trip failed", err)
	}
	for _, key := range [][]byte{derivePasswordKey("other-machine", "test-user"), derivePasswordKey("test-machine", "other-user")} {
		if _, err := decryptPassword(first, key); err == nil {
			t.Fatal("foreign machine/account accepted")
		}
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(first, passwordEnvelope))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	for _, invalid := range []string{"plain", passwordEnvelope + "!invalid", passwordEnvelope + "AA", "unknown:" + first, passwordEnvelope + base64.RawStdEncoding.EncodeToString(raw)} {
		if _, err := decryptPassword(invalid, key); err == nil {
			t.Fatal("invalid ciphertext accepted")
		}
	}
}

func TestPlaintextSettingsMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := []byte(`{"version":1,"proxy":{"sysProxy":true,"localPort":8899,"processNames":["curl.exe"]},"relay":{"port":8389,"password":"migration-secret"}}`)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	m := persistentTestManager(t, path)
	if m.SettingsError() != "" {
		t.Fatal(m.SettingsError())
	}
	if m.RelayOptions().Password != "migration-secret" || m.Options().LocalPort != 8899 {
		t.Fatal("migration lost preferences")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var disk savedSettings
	if err := json.Unmarshal(data, &disk); err != nil {
		t.Fatal(err)
	}
	if disk.Version != 2 || disk.Relay.LegacyPassword != nil || disk.Relay.PasswordEncrypted == "" || bytes.Contains(data, []byte("migration-secret")) {
		t.Fatal("migration did not remove plaintext")
	}
	loaded := persistentTestManager(t, path)
	if loaded.SettingsError() != "" || loaded.RelayOptions() != m.RelayOptions() {
		t.Fatal("migrated settings not readable")
	}
	// A foreign account cannot silently overwrite the encrypted original.
	foreign := NewManager()
	foreign.passwordKey = func() ([]byte, error) { return derivePasswordKey("other", "other"), nil }
	foreign.settingsErr = foreign.loadSettings(path)
	if foreign.SettingsError() == "" {
		t.Fatal("foreign settings accepted")
	}
	if err := foreign.SetRelayOptions(RelayOptions{Port: 8388, Password: "replacement"}); err == nil {
		t.Fatal("foreign settings overwritten")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, data) {
		t.Fatal("original ciphertext changed")
	}
}

func TestPasswordIdentityFailureNeverFallsBackToPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := []byte(`{"version":1,"relay":{"port":8388,"password":"private"}}`)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	m.passwordKey = func() ([]byte, error) { return nil, errors.New("identity unavailable") }
	m.settingsErr = m.loadSettings(path)
	if m.SettingsError() == "" {
		t.Fatal("migration failure ignored")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, legacy) {
		t.Fatal("failed migration changed original")
	}
	// Empty passwords need no identity. A failed subsequent encryption must
	// retain the old settings, with no plaintext or temporary file left behind.
	m.settingsErr = nil
	m.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := m.SetRelayOptions(RelayOptions{Port: 8388}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.settingsPath)
	if err := m.SetRelayOptions(RelayOptions{Port: 9000, Password: "private"}); err == nil {
		t.Fatal("identity failure ignored")
	}
	if after, _ := os.ReadFile(m.settingsPath); !bytes.Equal(after, before) {
		t.Fatal("encryption failure changed file")
	}
	if m.RelayOptions().Password != "" || m.RelayOptions().Port != 8388 {
		t.Fatal("encryption failure changed memory")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(m.settingsPath), ".settings-*.tmp"))
	if len(files) != 0 {
		t.Fatal("temporary file leaked")
	}
}
