// Package sockfilter implements opt-in, process-scoped transparent proxying.
package sockfilter

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf16"
)

// The SDK uses #pragma pack(1). Byte arrays deliberately avoid Go's padding.
const ruleSize = 643 // sizeof(NF_RULE_EX), nfsdk/include/nfdriver_data.h
type rule [ruleSize]byte
type address [28]byte

func NormalizeNames(names []string) ([]string, error) {
	result := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if !strings.HasSuffix(name, ".exe") || len(name) <= 4 || strings.ContainsAny(name, `\/":*?<>|`) || strings.ContainsFunc(name, unicode.IsControl) || len(utf16.Encode([]rune(name))) > 250 {
			return nil, fmt.Errorf("进程请填写完整文件名（如 chrome.exe），不支持路径或通配符: %q", name)
		}
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}
	if len(result) > 128 {
		return nil, fmt.Errorf("最多指定 128 个进程名")
	}
	return result, nil
}

func processRule(name string, protocol uint32, flags uint32) rule {
	var r rule
	binary.LittleEndian.PutUint32(r[0:4], protocol)
	if protocol == 6 {
		r[8] = 2
	} // NF_D_OUT; UDP rules apply at socket creation.
	binary.LittleEndian.PutUint32(r[79:83], flags)
	// The SDK matches a path suffix. Include the separator: chrome.exe must
	// not match not-chrome.exe. Names are validated before reaching here.
	for i, c := range utf16.Encode([]rune(`*\` + name)) {
		binary.LittleEndian.PutUint16(r[83+i*2:], c)
	}
	return r
}

func makeRules(names []string, pid uint32) []rule {
	var own rule
	binary.LittleEndian.PutUint32(own[4:8], pid)
	rules := []rule{own} // NF_ALLOW, even if the user selects this executable.
	for _, name := range names {
		rules = append(rules, processRule(name, 6, 16), processRule(name, 17, 2))
	}
	return append(rules, rule{}) // Explicit default bypass.
}

func (a address) addrPort() (netip.AddrPort, error) {
	var ip netip.Addr
	switch binary.LittleEndian.Uint16(a[:2]) {
	case 2:
		ip = netip.AddrFrom4([4]byte(a[4:8]))
	case 23:
		ip = netip.AddrFrom16([16]byte(a[8:24]))
	default:
		return netip.AddrPort{}, fmt.Errorf("unsupported address family")
	}
	return netip.AddrPortFrom(ip, binary.BigEndian.Uint16(a[2:4])), nil
}

func fromAddrPort(ap netip.AddrPort) address {
	var a address
	ip := ap.Addr()
	if ip.Is4() {
		a[0] = 2
		v := ip.As4()
		copy(a[4:8], v[:])
	} else {
		a[0] = 23
		v := ip.As16()
		copy(a[8:24], v[:])
	}
	binary.BigEndian.PutUint16(a[2:4], ap.Port())
	return a
}

// Do not intercept local IPC, multicast, broadcast or link-local traffic.
// Private unicast addresses are still proxied when a selected app uses them.
func bypass(ap netip.AddrPort) bool {
	ip := ap.Addr().Unmap()
	return !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}
