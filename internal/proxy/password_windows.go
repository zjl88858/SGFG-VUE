//go:build windows

package proxy

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func localPasswordKey() ([]byte, error) {
	failure := errors.New("无法读取本机/账户标识，不能加密或解密 SS 中继密码")
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil, failure
	}
	defer k.Close()
	machineID, _, err := k.GetStringValue("MachineGuid")
	if err != nil || strings.TrimSpace(machineID) == "" {
		return nil, failure
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user.User.Sid == nil {
		return nil, failure
	}
	return derivePasswordKey(strings.ToLower(strings.TrimSpace(machineID)), user.User.Sid.String()), nil
}
