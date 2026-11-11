//go:build windows

package main

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type startupIdentity struct {
	userSID   string
	sessionID uint32
	elevated  bool
}

var (
	errStartupAccountMismatch   = errors.New("SGFG 与当前桌面不属于同一账户或会话。请返回桌面直接启动 SGFG，避免以其他账户或管理员保护的临时账户运行。")
	errStartupElevationMismatch = errors.New("请从当前桌面直接启动 SGFG，不要单独选择“以管理员身份运行”。进程代理会通过独立的驱动助手请求授权。")
)

// Compare actual token identities, not account names, ADMIN_* prefixes, OS
// versions or UAC policy. Built-in Administrator and UAC-disabled desktops may
// already run elevated, without any lower-privilege desktop to launch from.
func checkGUIStartupIdentity(current startupIdentity, readDesktop func() (startupIdentity, error)) error {
	if !current.elevated {
		return nil
	}
	desktop, err := readDesktop()
	if err != nil {
		return fmt.Errorf("无法确认当前桌面账户，请等待桌面加载完成后重新启动 SGFG。\n\n%w", err)
	}
	if current.userSID == "" || current.userSID != desktop.userSID ||
		current.sessionID == 0 || current.sessionID != desktop.sessionID {
		return errStartupAccountMismatch
	}
	if !desktop.elevated {
		return errStartupElevationMismatch
	}
	return nil
}

func startupTokenUint32(token windows.Token, class uint32) (uint32, error) {
	var value, size uint32
	if err := windows.GetTokenInformation(token, class, (*byte)(unsafe.Pointer(&value)), uint32(unsafe.Sizeof(value)), &size); err != nil {
		return 0, err
	}
	if size != uint32(unsafe.Sizeof(value)) {
		return 0, fmt.Errorf("unexpected token information size: %d", size)
	}
	return value, nil
}

func readStartupIdentity(token windows.Token) (startupIdentity, error) {
	// IsElevated returns false on query errors; handle errors explicitly here so
	// a failed token query cannot be mistaken for an ordinary-user launch.
	elevated, err := startupTokenUint32(token, windows.TokenElevation)
	if err != nil {
		return startupIdentity{}, fmt.Errorf("TokenElevation: %w", err)
	}
	user, err := token.GetTokenUser()
	if err != nil {
		return startupIdentity{}, fmt.Errorf("TokenUser: %w", err)
	}
	session, err := startupTokenUint32(token, windows.TokenSessionId)
	if err != nil {
		return startupIdentity{}, fmt.Errorf("TokenSessionId: %w", err)
	}
	return startupIdentity{userSID: user.User.Sid.String(), sessionID: session, elevated: elevated != 0}, nil
}

func desktopStartupIdentity() (startupIdentity, error) {
	// GetShellWindow identifies this desktop's shell, including an RDP desktop.
	// Do not pick an arbitrary explorer.exe or the active console session.
	shell := windows.GetShellWindow()
	if shell == 0 {
		return startupIdentity{}, errors.New("当前桌面 Shell 尚未就绪")
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(shell, &pid); err != nil {
		return startupIdentity{}, fmt.Errorf("GetWindowThreadProcessId: %w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return startupIdentity{}, fmt.Errorf("OpenProcess(desktop): %w", err)
	}
	defer windows.CloseHandle(process)
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return startupIdentity{}, fmt.Errorf("OpenProcessToken(desktop): %w", err)
	}
	defer token.Close()
	identity, err := readStartupIdentity(token)
	if err != nil {
		return startupIdentity{}, err
	}
	// If the shell restarted while its token was being read, do not use a stale
	// desktop identity to authorize an elevated GUI.
	var currentPID uint32
	if windows.GetShellWindow() != shell {
		return startupIdentity{}, errors.New("桌面 Shell 已重新启动")
	}
	if _, err := windows.GetWindowThreadProcessId(shell, &currentPID); err != nil || currentPID != pid {
		return startupIdentity{}, errors.New("桌面 Shell 已更改")
	}
	return identity, nil
}
