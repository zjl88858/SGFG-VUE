//go:build !windows && !darwin && !linux

package sysproxy

import "fmt"

type unsupportedController struct{}

func newController() Controller { return &unsupportedController{} }
func (*unsupportedController) Available() (bool, string) {
	return false, "当前平台不支持自动系统代理"
}
func (*unsupportedController) Enable(string) error {
	return fmt.Errorf("当前平台不支持自动系统代理")
}
func (*unsupportedController) Restore() error { return nil }
