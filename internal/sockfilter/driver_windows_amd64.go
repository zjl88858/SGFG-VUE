//go:build windows && amd64

package sockfilter

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const driverDisplayName = "SGFG Process Proxy Driver"

type scmDriver struct {
	manager        *mgr.Mgr
	service        *mgr.Service
	guard          windows.Handle
	source, target string
}

func acquireDriver() (*driverLease, error) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return nil, fmt.Errorf("驱动 helper 未获得管理员权限，请允许进程代理的 UAC 请求")
	}
	// A named event is a cross-process ownership guard with no OS-thread
	// affinity. Do not unload a driver while another SGFG instance owns it.
	name, _ := windows.UTF16PtrFromString(`Global\SGFG.SockFilter.Owner.v1`)
	h, err := windows.CreateEvent(nil, 0, 0, name)
	if err != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return nil, fmt.Errorf("进程代理正由另一个应用实例使用")
		}
		return nil, fmt.Errorf("无法锁定进程代理驱动: %w", err)
	}
	m, err := mgr.Connect()
	if err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("无法访问驱动服务管理器: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		m.Disconnect()
		windows.CloseHandle(h)
		return nil, err
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		m.Disconnect()
		windows.CloseHandle(h)
		return nil, err
	}
	b := &scmDriver{manager: m, guard: h,
		source: filepath.Join(filepath.Dir(exe), "sockfilter", "driver", "x64", "sockfilter.sys"),
		target: filepath.Join(system, "drivers", managedDriverName+".sys")}
	return startManagedDriver(b)
}

func validateManagedService(c mgr.Config, target string) error {
	p := strings.Trim(c.BinaryPathName, `"`)
	p = strings.TrimPrefix(p, `\??\`)
	if c.ServiceType != windows.SERVICE_KERNEL_DRIVER || c.DisplayName != driverDisplayName ||
		!strings.EqualFold(filepath.Clean(p), filepath.Clean(target)) {
		return fmt.Errorf("同名驱动服务不属于本应用，已取消操作以保护现有配置")
	}
	return nil
}

func (b *scmDriver) Install() error {
	data, err := os.ReadFile(b.source)
	if err != nil {
		return fmt.Errorf("缺少进程代理驱动文件: %w", err)
	}
	// Refuse to overwrite a different driver at the reserved path. SCM also
	// validates the Windows kernel signature when Start is requested.
	if existing, err := os.ReadFile(b.target); err == nil {
		if sha256.Sum256(existing) != sha256.Sum256(data) {
			return fmt.Errorf("驱动文件与应用版本不一致，请先卸载旧版 SGFG 进程驱动")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	s, err := b.manager.OpenService(managedDriverName)
	if err == nil {
		b.service = s
		c, err := s.Config()
		if err != nil {
			return err
		}
		if err = validateManagedService(c, b.target); err != nil {
			return err
		}
		// Recover an owned demand-start driver left behind by an abnormal exit.
		if err = b.Stop(); err != nil {
			return err
		}
		if err = b.Remove(); err != nil {
			return err
		}
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("读取驱动服务失败: %w", err)
	}
	f, err := os.OpenFile(b.target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	createdFile := err == nil
	if err != nil {
		// An identical orphan file can remain if a prior file removal failed.
		// Its hash was checked above; reuse it, never overwrite it in place.
		if !os.IsExist(err) {
			return fmt.Errorf("安装驱动文件失败: %w", err)
		}
	} else {
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if err = errors.Join(writeErr, closeErr); err != nil {
			return errors.Join(err, os.Remove(b.target))
		}
	}
	s, err = b.manager.CreateService(managedDriverName, b.target, mgr.Config{
		ServiceType: windows.SERVICE_KERNEL_DRIVER, StartType: windows.SERVICE_DEMAND_START,
		ErrorControl: windows.SERVICE_ERROR_NORMAL, DisplayName: driverDisplayName,
	})
	if err != nil {
		if createdFile {
			return errors.Join(fmt.Errorf("注册进程驱动失败: %w", err), os.Remove(b.target))
		}
		return fmt.Errorf("注册进程驱动失败: %w", err)
	}
	b.service = s
	return nil
}

func (b *scmDriver) wait(state svc.State) error {
	deadline := time.Now().Add(8 * time.Second)
	for {
		s, err := b.service.Query()
		if err != nil {
			return err
		}
		if s.State == state {
			return nil
		}
		if state == svc.Running && s.State == svc.Stopped {
			return fmt.Errorf("驱动未能启动（Windows 错误 %d）", s.Win32ExitCode)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待驱动状态超时（当前状态 %d）", s.State)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (b *scmDriver) Start() error {
	if err := b.service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("加载进程驱动失败，请检查驱动签名及系统安全策略: %w", err)
	}
	return b.wait(svc.Running)
}

func (b *scmDriver) Stop() error {
	if b.service == nil {
		return nil
	}
	s, err := b.service.Query()
	if err != nil {
		return err
	}
	if s.State == svc.Stopped {
		return nil
	}
	if s.State != svc.StopPending {
		if _, err = b.service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return fmt.Errorf("卸载进程驱动失败，驱动可能仍被占用: %w", err)
		}
	}
	return b.wait(svc.Stopped)
}

func (b *scmDriver) Remove() error {
	if b.service != nil {
		if err := b.service.Delete(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
			return fmt.Errorf("删除驱动服务失败: %w", err)
		}
		b.service.Close()
		b.service = nil
		deadline := time.Now().Add(5 * time.Second)
		for {
			s, err := b.manager.OpenService(managedDriverName)
			if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
				break
			}
			if err == nil {
				s.Close()
			} else if !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
				return err
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("驱动服务正在删除，请关闭其他服务管理窗口后重试")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err := os.Remove(b.target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除驱动文件失败: %w", err)
	}
	return nil
}

func (b *scmDriver) Close() {
	if b.service != nil {
		b.service.Close()
		b.service = nil
	}
	if b.manager != nil {
		b.manager.Disconnect()
		b.manager = nil
	}
	if b.guard != 0 {
		windows.CloseHandle(b.guard)
		b.guard = 0
	}
}
