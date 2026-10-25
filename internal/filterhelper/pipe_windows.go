//go:build windows && amd64

package filterhelper

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const pipePrefix = `\\.\pipe\SGFG.Filter.v1.`

func createPipe(name string) (windows.Handle, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return 0, err
	}
	// Admin Protection may elevate under a different temporary account. Permit
	// administrators in the ACL, then authenticate the exact launched PID below.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;" + user.User.Sid.String() + ")S:(ML;;NW;;;ME)")
	if err != nil {
		return 0, err
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	return windows.CreateNamedPipe(p, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		1, 65536, 65536, 0, sa)
}

// Complete connection before handing the overlapped pipe to Go's os.File
// poller. os.NewFile recognizes overlapped pipes and supports I/O deadlines.
func acceptPipe(h, child windows.Handle) error {
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event)
	ov := windows.Overlapped{HEvent: event}
	err = windows.ConnectNamedPipe(h, &ov)
	if err == nil || errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		return nil
	}
	if !errors.Is(err, windows.ERROR_IO_PENDING) {
		return fmt.Errorf("ConnectNamedPipe: %w", err)
	}
	result, waitErr := windows.WaitForMultipleObjects([]windows.Handle{event, child}, false, 10000)
	if waitErr == nil && result == windows.WAIT_OBJECT_0 {
		var n uint32
		if err := windows.GetOverlappedResult(h, &ov, &n, false); err != nil {
			return fmt.Errorf("GetOverlappedResult: %w", err)
		}
		return nil
	}
	// Drain cancelled overlapped I/O before its stack storage can be released.
	_ = windows.CancelIoEx(h, &ov)
	var n uint32
	_ = windows.GetOverlappedResult(h, &ov, &n, true)
	if waitErr != nil {
		return fmt.Errorf("WaitForMultipleObjects: %w", waitErr)
	}
	return fmt.Errorf("驱动 helper 未连接（退出或连接超时）")
}

func dialPipe(name string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			return os.NewFile(uintptr(h), name), nil
		}
		if (!errors.Is(err, windows.ERROR_PIPE_BUSY) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND)) || time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(50 * time.Millisecond)
	}
}
