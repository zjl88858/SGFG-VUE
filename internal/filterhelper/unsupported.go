//go:build !windows || !amd64

package filterhelper

import (
	"errors"
	"sgfg-ng/internal/sockfilter"
)

func Start(int, []string, func(string, string), sockfilter.Options) (*Client, error) {
	return nil, errors.New("进程代理 helper 当前仅支持 Windows x64")
}
