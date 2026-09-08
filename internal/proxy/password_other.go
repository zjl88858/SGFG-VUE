//go:build !windows

package proxy

import "errors"

func localPasswordKey() ([]byte, error) {
	return nil, errors.New("本机绑定的密码存储当前仅支持 Windows")
}
