//go:build windows

package sysproxy

import (
	"fmt"
	"sync"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const internetSettings = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

var (
	wininet            = syscall.NewLazyDLL("wininet.dll")
	internetSetOptionW = wininet.NewProc("InternetSetOptionW")
)

type windowsSnapshot struct {
	enabled     uint64
	server      string
	override    string
	hadEnabled  bool
	hadServer   bool
	hadOverride bool
}

type windowsController struct {
	mu       sync.Mutex
	snapshot *windowsSnapshot
}

func newController() Controller { return &windowsController{} }

func (*windowsController) Available() (bool, string) { return true, "Windows WinINET" }

func (c *windowsController) Enable(server string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, internetSettings, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if c.snapshot == nil {
		snapshot := &windowsSnapshot{}
		if value, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
			snapshot.enabled, snapshot.hadEnabled = value, true
		}
		if value, _, err := key.GetStringValue("ProxyServer"); err == nil {
			snapshot.server, snapshot.hadServer = value, true
		}
		if value, _, err := key.GetStringValue("ProxyOverride"); err == nil {
			snapshot.override, snapshot.hadOverride = value, true
		}
		c.snapshot = snapshot
	}
	if err := key.SetStringValue("ProxyServer", server); err != nil {
		return err
	}
	if err := key.SetStringValue("ProxyOverride", "<local>"); err != nil {
		return err
	}
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	notifyWindows()
	return nil
}

func (c *windowsController) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot == nil {
		return nil
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, internetSettings, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	snapshot := c.snapshot
	if snapshot.hadEnabled {
		err = key.SetDWordValue("ProxyEnable", uint32(snapshot.enabled))
	} else {
		err = ignoreMissing(key.DeleteValue("ProxyEnable"))
	}
	if err == nil {
		if snapshot.hadServer {
			err = key.SetStringValue("ProxyServer", snapshot.server)
		} else {
			err = ignoreMissing(key.DeleteValue("ProxyServer"))
		}
	}
	if err == nil {
		if snapshot.hadOverride {
			err = key.SetStringValue("ProxyOverride", snapshot.override)
		} else {
			err = ignoreMissing(key.DeleteValue("ProxyOverride"))
		}
	}
	if err != nil {
		return fmt.Errorf("restore WinINET proxy: %w", err)
	}
	c.snapshot = nil
	notifyWindows()
	return nil
}

func ignoreMissing(err error) error {
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func notifyWindows() {
	internetSetOptionW.Call(0, 39, 0, 0)
	internetSetOptionW.Call(0, 37, 0, 0)
}
