package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type RelayOptions struct {
	Port     int    `json:"port"`
	Password string `json:"password"`
}

// An explicit allowlist: runtime state, node selection and traffic never enter
// the configuration file. Relay activation is also a runtime-only action.
type savedSettings struct {
	Version int               `json:"version"`
	Proxy   ConnectOptions    `json:"proxy"`
	Relay   savedRelayOptions `json:"relay"`
}

type savedRelayOptions struct {
	Port              int    `json:"port"`
	PasswordEncrypted string `json:"passwordEncrypted,omitempty"`
	// Read only, for migration. New saves never populate this field.
	LegacyPassword *string `json:"password,omitempty"`
}

func NewPersistentManager() *Manager {
	m := NewManager()
	dir, err := os.UserConfigDir()
	if err == nil {
		err = m.loadSettings(filepath.Join(dir, "SGFG", "settings.json"))
	}
	m.settingsErr = err
	return m
}

// Called before the manager is exposed to other goroutines.
func (m *Manager) loadSettings(path string) error {
	m.settingsPath = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取本地设置失败: %w", err)
	}
	s := savedSettings{Version: 1, Proxy: m.opts, Relay: savedRelayOptions{Port: m.relayOpts.Port}}
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("本地设置文件格式无效，请修复或移走 %s 后重新启动", path)
	}
	if s.Version != 1 && s.Version != 2 {
		return fmt.Errorf("不支持的设置文件版本 %d", s.Version)
	}
	opts, err := normalizeOptions(s.Proxy)
	if err != nil {
		return fmt.Errorf("本地代理设置无效: %w", err)
	}
	relay := RelayOptions{Port: s.Relay.Port}
	if err := validateRelayOptions(relay); err != nil {
		return err
	}
	if s.Version == 1 {
		if s.Relay.PasswordEncrypted != "" {
			return errors.New("旧版设置中包含不兼容的加密字段")
		}
		if s.Relay.LegacyPassword != nil {
			relay.Password = *s.Relay.LegacyPassword
		}
		// Migrate before accepting the preferences. Failure leaves the original
		// file intact and surfaces the standard startup settings error.
		if err := m.saveSettingsLocked(opts, relay); err != nil {
			return fmt.Errorf("旧版密码加密迁移失败: %w", err)
		}
	} else {
		if s.Relay.LegacyPassword != nil {
			return errors.New("新版设置不接受明文密码字段")
		}
		if s.Relay.PasswordEncrypted != "" {
			key, err := m.passwordKey()
			if err != nil {
				return err
			}
			relay.Password, err = decryptPassword(s.Relay.PasswordEncrypted, key)
			if err != nil {
				return err
			}
		}
	}
	m.opts, m.relayOpts = opts, relay
	return nil
}

func (m *Manager) SettingsError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settingsErr != nil {
		return m.settingsErr.Error()
	}
	return ""
}

func (m *Manager) saveSettingsLocked(opts ConnectOptions, relay RelayOptions) error {
	if m.settingsErr != nil {
		return fmt.Errorf("无法保存设置: %w", m.settingsErr)
	}
	if m.settingsPath == "" {
		return nil
	} // In-memory manager for tests/embedding.
	diskRelay := savedRelayOptions{Port: relay.Port}
	if relay.Password != "" {
		key, err := m.passwordKey()
		if err != nil {
			return err
		}
		diskRelay.PasswordEncrypted, err = encryptPassword(relay.Password, key)
		if err != nil {
			return fmt.Errorf("加密 SS 中继密码失败: %w", err)
		}
	}
	data, err := json.MarshalIndent(savedSettings{Version: 2, Proxy: opts, Relay: diskRelay}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeSettings(m.settingsPath, append(data, '\n')); err != nil {
		return fmt.Errorf("保存本地设置失败: %w", err)
	}
	return nil
}

func writeSettings(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	// Same-directory replacement preserves the previous file on write failure.
	return os.Rename(tmp, path)
}

func validateRelayOptions(opts RelayOptions) error {
	if opts.Port < 1 || opts.Port > 65535 {
		return errors.New("中继端口需在 1-65535 之间")
	}
	return nil // An empty saved password is allowed; starting the relay requires one.
}

func (m *Manager) RelayOptions() RelayOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.relayOpts
}

func (m *Manager) SetRelayOptions(opts RelayOptions) error {
	if err := validateRelayOptions(opts); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.relayOn {
		return errors.New("请先关闭 SS 中继再修改中继设置")
	}
	if err := m.saveSettingsLocked(m.opts, opts); err != nil {
		return err
	}
	m.relayOpts = opts
	return nil
}
