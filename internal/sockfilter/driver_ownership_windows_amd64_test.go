//go:build windows && amd64

package sockfilter

import (
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestManagedDriverOwnership(t *testing.T) {
	path := `C:\Windows\System32\drivers\sgfg_sockfilter.sys`
	good := mgr.Config{ServiceType: windows.SERVICE_KERNEL_DRIVER, DisplayName: driverDisplayName, BinaryPathName: path}
	if err := validateManagedService(good, path); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*mgr.Config){
		func(c *mgr.Config) { c.DisplayName = "Other software" },
		func(c *mgr.Config) { c.BinaryPathName = `C:\other.sys` },
		func(c *mgr.Config) { c.ServiceType = windows.SERVICE_WIN32_OWN_PROCESS },
	} {
		c := good
		change(&c)
		if validateManagedService(c, path) == nil {
			t.Fatal("foreign service accepted")
		}
	}
}
