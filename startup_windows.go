//go:build windows

package main

import "golang.org/x/sys/windows"

func normalUserStartup() bool {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return true
	}
	// asInvoker cannot undo an explicitly elevated launch. Refuse before Wails,
	// configuration or HKCU proxy access to avoid using ADMIN_* under protection.
	text, _ := windows.UTF16PtrFromString("请以普通用户方式启动 SGFG，不要选择“以管理员身份运行”。\n\n主界面和系统代理必须使用当前用户身份。开启进程代理后，应用会单独为驱动 helper 请求管理员授权。")
	title, _ := windows.UTF16PtrFromString("请正常启动 SGFG")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONWARNING)
	return false
}
