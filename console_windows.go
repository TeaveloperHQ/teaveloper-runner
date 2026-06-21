package main

import "syscall"

// isConsole 은 콘솔 창이 붙어 있는지 본다. -H windowsgui 로 빌드하면 콘솔이 없어
// false 가 된다(그때는 파일 로그만).
func isConsole() bool {
	k := syscall.NewLazyDLL("kernel32.dll")
	r, _, _ := k.NewProc("GetConsoleWindow").Call()
	return r != 0
}
