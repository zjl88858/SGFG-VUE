//go:build windows && amd64

package filterhelper

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/windows"
	"sgfg-ng/internal/sockfilter"
)

func Start(port int, names []string, log func(string, string), options sockfilter.Options) (*Client, error) {
	request := message{Type: "start", Port: port, Names: names, TCPOnly: options.TCPOnly}
	if err := validateStart(request); err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	helper := filepath.Join(filepath.Dir(exe), helperName)
	if fi, err := os.Stat(helper); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("缺少驱动 helper，请保留主程序同目录的 %s", helperName)
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	name := pipePrefix + hex.EncodeToString(nonce[:])
	h, err := createPipe(name)
	if err != nil {
		return nil, fmt.Errorf("创建驱动控制管道失败: %w", err)
	}
	owned := true
	defer func() {
		if owned {
			windows.CloseHandle(h)
		}
	}()
	if log != nil {
		log("info", "正在请求进程代理授权，请确认 Windows 提示")
	}
	process, err := elevate(helper, "--pipe "+name+" --parent "+strconv.Itoa(os.Getpid()))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(process)
	var waitHandle windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), process, windows.CurrentProcess(), &waitHandle, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, err
	}
	exited := make(chan struct{})
	go func() {
		_, _ = windows.WaitForSingleObject(waitHandle, windows.INFINITE)
		windows.CloseHandle(waitHandle)
		close(exited)
	}()
	// Do not reuse a PID from a process snapshot: this is the handle returned
	// by ShellExecuteEx for this specific UAC launch.
	pid, err := windows.GetProcessId(process)
	if err == nil {
		err = acceptPipe(h, process)
	}
	if err == nil {
		err = verifyPeer(h, pid, false)
	}
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), name)
	owned = false
	c, err := startClient(f, request, log)
	if err != nil {
		select {
		case <-exited:
		case <-time.After(cleanupTimeout):
		}
		return nil, err
	}
	c.processDone = exited
	return c, nil
}
