//go:build windows && amd64

package filterhelper

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SHELLEXECUTEINFOW, Windows x64. No script interpreter or command shell.
type shellExecuteInfo struct {
	Size                              uint32
	Mask                              uint32
	Window                            windows.Handle
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance                          windows.Handle
	IDList                            uintptr
	Class                             *uint16
	ClassKey                          windows.Handle
	HotKey                            uint32
	Icon                              windows.Handle
	Process                           windows.Handle
}

func elevate(exe, arguments string) (windows.Handle, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err == nil || errors.Is(err, syscall.Errno(windows.S_FALSE)) {
		defer windows.CoUninitialize()
	} else if !errors.Is(err, syscall.Errno(windows.RPC_E_CHANGED_MODE)) {
		return 0, err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	args, err := windows.UTF16PtrFromString(arguments)
	if err != nil {
		return 0, err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return 0, err
	}
	info := shellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Verb: verb, File: file, Parameters: args, Directory: dir, Show: windows.SW_HIDE}
	info.Size = uint32(unsafe.Sizeof(info))
	p := windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")
	ok, _, callErr := p.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return 0, errors.New("已取消进程代理授权；普通代理仍可使用")
		}
		return 0, fmt.Errorf("请求驱动 helper 管理员权限失败: %w", callErr)
	}
	if info.Process == 0 {
		return 0, errors.New("未能取得驱动 helper 进程句柄")
	}
	return info.Process, nil
}

func processPath(h windows.Handle) (string, error) {
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(windows.UTF16ToString(buf[:n]))
}

func verifyPeer(pipe windows.Handle, expectedPID uint32, server bool) error {
	var actual uint32
	var err error
	if server {
		err = windows.GetNamedPipeServerProcessId(pipe, &actual)
	} else {
		err = windows.GetNamedPipeClientProcessId(pipe, &actual)
	}
	if err != nil {
		return err
	}
	if actual != expectedPID {
		return errors.New("驱动 helper 管道身份校验失败")
	}
	return nil
}

func verifyParent(pipe windows.Handle, parentPID uint32, self string) (windows.Handle, error) {
	if err := verifyPeer(pipe, parentPID, true); err != nil {
		return 0, err
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, parentPID)
	if err != nil {
		return 0, err
	}
	valid := false
	defer func() {
		if !valid {
			windows.CloseHandle(h)
		}
	}()
	parentPath, err := processPath(h)
	if err != nil {
		return 0, err
	}
	selfPath, err := filepath.EvalSymlinks(self)
	if err != nil {
		return 0, err
	}
	if !strings.EqualFold(filepath.Dir(parentPath), filepath.Dir(selfPath)) || strings.EqualFold(parentPath, selfPath) {
		return 0, errors.New("驱动 helper 必须由同目录的主程序启动")
	}
	var parentSession, selfSession uint32
	if err := windows.ProcessIdToSessionId(parentPID, &parentSession); err != nil {
		return 0, err
	}
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &selfSession); err != nil {
		return 0, err
	}
	if parentSession != selfSession {
		return 0, errors.New("驱动 helper 与主程序不属于同一登录会话")
	}
	if state, err := windows.WaitForSingleObject(h, 0); err != nil || state != uint32(windows.WAIT_TIMEOUT) {
		return 0, errors.New("主程序已经退出")
	}
	valid = true
	return h, nil
}
