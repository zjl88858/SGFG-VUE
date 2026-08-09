//go:build linux

package sysproxy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const gnomeProxySchema = "org.gnome.system.proxy"

var gnomeProxyKeys = [][2]string{
	{gnomeProxySchema, "mode"},
	{gnomeProxySchema + ".http", "host"},
	{gnomeProxySchema + ".http", "port"},
	{gnomeProxySchema + ".https", "host"},
	{gnomeProxySchema + ".https", "port"},
}

type linuxController struct {
	mu       sync.Mutex
	snapshot []string
}

func newController() Controller { return &linuxController{} }

func (*linuxController) Available() (bool, string) {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return false, "Linux 无统一系统代理，且未找到 GNOME gsettings"
	}
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + ":" + os.Getenv("DESKTOP_SESSION"))
	for _, name := range []string{"gnome", "unity", "cinnamon", "pantheon", "budgie"} {
		if strings.Contains(desktop, name) {
			return true, "GNOME gsettings"
		}
	}
	return false, "当前 Linux 桌面不支持自动系统代理，请手动配置"
}

func (c *linuxController) Enable(server string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	host, portText, err := net.SplitHostPort(server)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return err
	}
	if c.snapshot == nil {
		for _, key := range gnomeProxyKeys {
			value, err := gsettings("get", key[0], key[1])
			if err != nil {
				return err
			}
			c.snapshot = append(c.snapshot, value)
		}
	}
	commands := [][3]string{
		{gnomeProxySchema + ".http", "host", strconv.Quote(host)},
		{gnomeProxySchema + ".http", "port", strconv.Itoa(port)},
		{gnomeProxySchema + ".https", "host", strconv.Quote(host)},
		{gnomeProxySchema + ".https", "port", strconv.Itoa(port)},
		{gnomeProxySchema, "mode", "'manual'"},
	}
	for _, command := range commands {
		if _, err := gsettings("set", command[0], command[1], command[2]); err != nil {
			return err
		}
	}
	return nil
}

func (c *linuxController) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, value := range c.snapshot {
		key := gnomeProxyKeys[i]
		if _, err := gsettings("set", key[0], key[1], value); err != nil {
			return err
		}
	}
	c.snapshot = nil
	return nil
}

func gsettings(args ...string) (string, error) {
	output, err := exec.Command("gsettings", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gsettings %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
