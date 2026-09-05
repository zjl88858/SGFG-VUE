//go:build !windows || !amd64

package sockfilter

import "fmt"

type Session struct{}

func Start(port int, names []string, log func(string, string), options ...Options) (*Session, error) {
	return nil, fmt.Errorf("SockFilter 当前仅支持 Windows x64 构建")
}
func (*Session) Close() error { return nil }
