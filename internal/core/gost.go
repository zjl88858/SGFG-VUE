package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-gost/core/logger"
	"github.com/go-gost/core/service"
	"github.com/go-gost/x/config"
	gostcmd "github.com/go-gost/x/config/cmd"
	chain_parser "github.com/go-gost/x/config/parsing/chain"
	logger_parser "github.com/go-gost/x/config/parsing/logger"
	service_parser "github.com/go-gost/x/config/parsing/service"
	"github.com/go-gost/x/registry"

	_ "github.com/go-gost/x/connector/http"
	_ "github.com/go-gost/x/connector/socks/v5"
	_ "github.com/go-gost/x/connector/ss"
	_ "github.com/go-gost/x/connector/ss/udp"
	_ "github.com/go-gost/x/dialer/tcp"
	_ "github.com/go-gost/x/dialer/udp"
	_ "github.com/go-gost/x/handler/auto"
	_ "github.com/go-gost/x/handler/http"
	_ "github.com/go-gost/x/handler/socks/v5"
	_ "github.com/go-gost/x/handler/ss"
	_ "github.com/go-gost/x/handler/ss/udp"
	_ "github.com/go-gost/x/listener/tcp"
	_ "github.com/go-gost/x/listener/udp"
)

var (
	gostMu     sync.Mutex
	gostLogger = &gostEventLogger{Logger: logger_parser.ParseLogger(&config.LoggerConfig{
		Log: &config.LogConfig{Level: gostLogLevel(), Output: "stderr"},
	})}
)

func init() {
	logger.SetDefault(gostLogger)
}

func gostLogLevel() string {
	if os.Getenv("SGFG_GOST_DEBUG") == "1" {
		return "debug"
	}
	return "info"
}

type GostInstance struct {
	id     string
	svcs   []service.Service
	chains []string
}

// GOST remains only for the SS LAN entrance forwarding to the local proxy.
// It has no WebTransport/remote-node role in v12.
func StartGost(listenURL, forwardURL string) (*GostInstance, error) {
	gostMu.Lock()
	defer gostMu.Unlock()

	nodes := []string{}
	if forwardURL != "" {
		nodes = []string{forwardURL}
	}
	cfg, err := gostcmd.BuildConfigFromCmd([]string{listenURL}, nodes)
	if err != nil {
		return nil, fmt.Errorf("build config: %w", err)
	}

	inst := &GostInstance{id: fmt.Sprintf("sgfg-%d", time.Now().UnixNano())}

	for _, c := range cfg.Chains {
		c.Name = inst.id + "-" + c.Name
		inst.chains = append(inst.chains, c.Name)
		for _, h := range c.Hops {
			h.Name = inst.id + "-" + h.Name
		}
	}
	for _, s := range cfg.Services {
		s.Name = inst.id + "-" + s.Name
		if s.Handler != nil && s.Handler.Chain != "" {
			s.Handler.Chain = inst.id + "-" + s.Handler.Chain
		}
		if s.Listener != nil && s.Listener.Chain != "" {
			s.Listener.Chain = inst.id + "-" + s.Listener.Chain
		}
	}

	for _, c := range cfg.Chains {
		ch, err := chain_parser.ParseChain(c, gostLogger)
		if err != nil {
			inst.cleanup()
			return nil, fmt.Errorf("parse chain %s: %w", c.Name, err)
		}
		if err := registry.ChainRegistry().Register(c.Name, ch); err != nil {
			inst.cleanup()
			return nil, fmt.Errorf("register chain %s: %w", c.Name, err)
		}
	}

	for _, s := range cfg.Services {
		svc, err := service_parser.ParseService(s)
		if err != nil {
			inst.cleanup()
			return nil, fmt.Errorf("parse service %s: %w", s.Name, err)
		}
		inst.svcs = append(inst.svcs, svc)
		go func() {
			if err := svc.Serve(); err != nil {
				fmt.Fprintf(os.Stderr, "gost service %s serve error: %v\n", s.Name, err)
			}
		}()
	}
	return inst, nil
}

func (g *GostInstance) Close() {
	gostMu.Lock()
	defer gostMu.Unlock()
	g.cleanup()
}

func (g *GostInstance) cleanup() {
	for _, svc := range g.svcs {
		svc.Close()
	}
	g.svcs = nil
	for _, name := range g.chains {
		registry.ChainRegistry().Unregister(name)
	}
	g.chains = nil
}
