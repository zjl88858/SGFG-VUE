//go:build darwin

package sysproxy

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type macProxy struct {
	enabled bool
	server  string
	port    string
}

type macService struct {
	name   string
	web    macProxy
	secure macProxy
}

type darwinController struct {
	mu       sync.Mutex
	snapshot []macService
}

func newController() Controller { return &darwinController{} }

func (*darwinController) Available() (bool, string) {
	if _, err := exec.LookPath("networksetup"); err != nil {
		return false, "networksetup 不可用"
	}
	return true, "macOS networksetup"
}

func (c *darwinController) Enable(server string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		return err
	}
	if _, err := strconv.Atoi(port); err != nil {
		return err
	}
	services, err := macServices()
	if err != nil {
		return err
	}
	if c.snapshot == nil {
		for _, service := range services {
			web, err := readMacProxy(service, "-getwebproxy")
			if err != nil {
				return err
			}
			secure, err := readMacProxy(service, "-getsecurewebproxy")
			if err != nil {
				return err
			}
			c.snapshot = append(c.snapshot, macService{name: service, web: web, secure: secure})
		}
	}
	for _, service := range services {
		if err := runNetworkSetup("-setwebproxy", service, host, port); err != nil {
			return err
		}
		if err := runNetworkSetup("-setsecurewebproxy", service, host, port); err != nil {
			return err
		}
	}
	return nil
}

func (c *darwinController) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, service := range c.snapshot {
		if err := restoreMacProxy(service.name, "-setwebproxy", "-setwebproxystate", service.web); err != nil {
			return err
		}
		if err := restoreMacProxy(service.name, "-setsecurewebproxy", "-setsecurewebproxystate", service.secure); err != nil {
			return err
		}
	}
	c.snapshot = nil
	return nil
}

func macServices() ([]string, error) {
	output, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var services []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "An asterisk") && !strings.HasPrefix(line, "*") {
			services = append(services, line)
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("没有可配置的网络服务")
	}
	return services, nil
}

func readMacProxy(service, flag string) (macProxy, error) {
	output, err := exec.Command("networksetup", flag, service).Output()
	if err != nil {
		return macProxy{}, err
	}
	proxy := macProxy{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Enabled":
			proxy.enabled = strings.EqualFold(strings.TrimSpace(value), "Yes")
		case "Server":
			proxy.server = strings.TrimSpace(value)
		case "Port":
			proxy.port = strings.TrimSpace(value)
		}
	}
	return proxy, scanner.Err()
}

func restoreMacProxy(service, setFlag, stateFlag string, proxy macProxy) error {
	if proxy.server != "" && proxy.port != "" {
		if err := runNetworkSetup(setFlag, service, proxy.server, proxy.port); err != nil {
			return err
		}
	}
	state := "off"
	if proxy.enabled {
		state = "on"
	}
	return runNetworkSetup(stateFlag, service, state)
}

func runNetworkSetup(args ...string) error {
	output, err := exec.Command("networksetup", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("networksetup %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return nil
}
