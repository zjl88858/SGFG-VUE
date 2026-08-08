package main

import (
	"embed"
	"log"
	"net"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"

	"sgfg-ng/internal/core"
	"sgfg-ng/internal/proxy"
	"sgfg-ng/internal/sysproxy"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	restoreStaleSysProxy()

	mgr := proxy.NewManager()
	svc := NewProxyService(mgr)

	app := application.New(application.Options{
		Name:        "sgfg-ng",
		Description: "神龟翻滚 NextGen",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		OnShutdown: func() {
			mgr.Shutdown()
		},
	})

	mgr.SetEmitter(func(name string, data ...any) {
		app.Event.Emit(name, data...)
	})
	svc.attachApp(app)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:        "神龟翻滚 SGFG-VUE " + Version,
		Width:        920,
		Height:       580,
		MinWidth:     860,
		MinHeight:    540,
		Frameless:    true,
		BackgroundColour: application.NewRGB(0xe8, 0xfa, 0xec),
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func restoreStaleSysProxy() {
	on, err := sysproxy.IsOn()
	if err != nil || !on {
		return
	}
	server, err := sysproxy.Server()
	if err != nil {
		return
	}
	host, portStr, err := net.SplitHostPort(server)
	if err != nil || host != "127.0.0.1" {
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return
	}
	if port >= core.LocalProxyPort && port < core.LocalProxyPort+core.LocalProxyPortMaxTry {
		sysproxy.Off()
	}
}
