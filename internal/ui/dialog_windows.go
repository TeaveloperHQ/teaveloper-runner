//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

var (
	user32      = syscall.NewLazyDLL("user32.dll")
	procMsgBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbIconInfo      = 0x00000040
	mbSetForeground = 0x00010000
)

// FatalError 는 콘솔이 없는 GUI 모드에서 오류를 메시지 박스로 보여준다.
func FatalError(title, body string) {
	messageBox(title, body, mbIconError)
}

// Info 는 안내 메시지 박스를 띄운다.
func Info(title, body string) {
	messageBox(title, body, mbIconInfo)
}

func messageBox(title, body string, icon uintptr) {
	t, _ := syscall.UTF16PtrFromString(title)
	b, _ := syscall.UTF16PtrFromString(body)
	procMsgBoxW.Call(
		0,
		uintptr(unsafe.Pointer(b)),
		uintptr(unsafe.Pointer(t)),
		mbOK|icon|mbSetForeground,
	)
}
