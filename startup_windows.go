//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func normalUserStartup() bool {
	current, err := readStartupIdentity(windows.GetCurrentProcessToken())
	if err != nil {
		err = fmt.Errorf("无法读取当前运行账户，请重新启动 SGFG。\n\n%w", err)
	} else {
		err = checkGUIStartupIdentity(current, desktopStartupIdentity)
	}
	if err == nil {
		return true
	}
	// Check before Wails, preferences and HKCU access. An already-elevated
	// interactive desktop is valid; elevation into a different profile is not.
	text, _ := windows.UTF16PtrFromString(err.Error())
	title, _ := windows.UTF16PtrFromString("无法启动 SGFG")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONWARNING)
	return false
}
