//go:build !windows

// 비 Windows(개발용) 빌드에서는 자동시작을 지원하지 않는다.
package autostart

import "errors"

var errUnsupported = errors.New("자동시작은 Windows 에서만 지원됩니다")

func Enabled() bool   { return false }
func Enable() error   { return errUnsupported }
func Disable() error  { return errUnsupported }
func Supported() bool { return false }
