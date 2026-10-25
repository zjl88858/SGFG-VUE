//go:build windows && amd64

package filterhelper

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"sgfg-ng/internal/sockfilter"
)

// Run is exclusively for the headless, elevated executable. It never creates
// Wails/WebView2, reads user settings, writes HKCU or selects a remote node.
func Run(pipe string, parentPID uint32) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("驱动 helper 需要管理员权限")
	}
	id := strings.TrimPrefix(pipe, pipePrefix)
	if !strings.HasPrefix(pipe, pipePrefix) || len(id) != 64 || parentPID == 0 {
		return errors.New("无效的 helper 启动参数")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return err
	}
	f, err := dialPipe(pipe)
	if err != nil {
		return err
	}
	defer f.Close()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	parent, err := verifyParent(windows.Handle(f.Fd()), parentPID, exe)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(parent)
	stop, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(stop)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		which, err := windows.WaitForMultipleObjects([]windows.Handle{parent, stop}, false, windows.INFINITE)
		if err != nil || which == windows.WAIT_OBJECT_0 {
			f.Close()
		}
	}()
	defer func() { windows.SetEvent(stop); <-watchDone }()
	return serve(f, parentPID, func(port int, names []string, log func(string, string), opts sockfilter.Options) (filterSession, error) {
		return sockfilter.Start(port, names, log, opts)
	})
}
