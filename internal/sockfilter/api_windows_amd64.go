//go:build windows && amd64

package sockfilter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

type sdk struct {
	dll   *windows.DLL
	procs map[string]*windows.Proc
}

var sessionMu sync.Mutex // NF SDK permits one attachment per driver.
var active atomic.Pointer[Session]
var callbackOnce sync.Once
var callbacks [16]uintptr // C ABI NF_EventHandler; static storage retained until process exit.

func loadSDK() (*sdk, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(exe), "sockfilter", "nfapi.dll")
	// Absolute path only: never search PATH or the current working directory.
	h, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return nil, fmt.Errorf("加载进程代理组件失败: %w；请确认应用目录中的 sockfilter/nfapi.dll 完整", err)
	}
	dll := &windows.DLL{Name: path, Handle: h}
	s := &sdk{dll: dll, procs: make(map[string]*windows.Proc)}
	for _, name := range []string{"nf_init", "nf_free", "nf_setOptions", "nf_setRulesEx", "nf_deleteRules", "nf_tcpPostSend", "nf_tcpPostReceive", "nf_udpPostSend", "nf_udpPostReceive", "nf_getDriverType"} {
		p, err := dll.FindProc(name)
		if err != nil {
			dll.Release()
			return nil, fmt.Errorf("不是兼容的 SockFilter C API DLL (%s): %w", name, err)
		}
		s.procs[name] = p
	}
	return s, nil
}

//go:uintptrescapes
func (s *sdk) call(name string, args ...uintptr) uintptr {
	r, _, _ := s.procs[name].Call(args...)
	return r
}

func (s *sdk) receiveUDP(id uintptr, a address, payload []byte) {
	var p unsafe.Pointer
	if len(payload) > 0 {
		p = unsafe.Pointer(&payload[0])
	}
	s.call("nf_udpPostReceive", id, uintptr(unsafe.Pointer(&a[0])), uintptr(p), uintptr(len(payload)), 0)
	runtime.KeepAlive(a)
	runtime.KeepAlive(payload)
}

func initCallbacks() {
	callbackOnce.Do(func() {
		cb := windows.NewCallbackCDecl // Windows x64 uses the unified calling convention.
		noop := cb(func() uintptr { return 0 })
		one := cb(func(id uintptr) uintptr { return 0 })
		two := cb(func(id uintptr, info unsafe.Pointer) uintptr { return 0 })
		callbacks = [16]uintptr{
			noop, noop,
			cb(func(id uintptr, info unsafe.Pointer) uintptr {
				if s := active.Load(); s != nil {
					s.tcpConnect(id, (*[67]byte)(info))
				}
				return 0
			}),
			two,
			cb(func(id uintptr, info unsafe.Pointer) uintptr {
				if s := active.Load(); s != nil {
					s.tcpClosed(id)
				}
				return 0
			}),
			cb(func(id uintptr, data unsafe.Pointer, size uint32) uintptr {
				if s := active.Load(); s != nil {
					s.api.call("nf_tcpPostReceive", id, uintptr(data), uintptr(size))
				}
				return 0
			}),
			cb(func(id uintptr, data unsafe.Pointer, size uint32) uintptr {
				if s := active.Load(); s != nil {
					s.api.call("nf_tcpPostSend", id, uintptr(data), uintptr(size))
				}
				return 0
			}),
			one, one, two, two,
			cb(func(id uintptr, info unsafe.Pointer) uintptr {
				if s := active.Load(); s != nil {
					s.udpClosed(id)
				}
				return 0
			}),
			cb(func(id uintptr, addr, data unsafe.Pointer, size uint32, opts unsafe.Pointer) uintptr {
				if s := active.Load(); s != nil {
					s.api.call("nf_udpPostReceive", id, uintptr(addr), uintptr(data), uintptr(size), uintptr(opts))
				}
				return 0
			}),
			cb(func(id uintptr, addr, data unsafe.Pointer, size uint32, opts unsafe.Pointer) uintptr {
				if s := active.Load(); s != nil {
					s.udpSend(id, (*address)(addr), data, uintptr(size), opts)
				}
				return 0
			}),
			one, one,
		}
	})
}

// Proxifier detection is advisory, not proof of driver conflict. No process
// is stopped and no third-party rules or services are modified.
func proxifierRunning() bool {
	h, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(h, &entry); err == nil; err = windows.Process32Next(h, &entry) {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if equalProcessName(name, "Proxifier.exe") || equalProcessName(name, "ProxifierPE.exe") {
			return true
		}
	}
	return false
}
