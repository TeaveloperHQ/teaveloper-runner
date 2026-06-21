package ui

import (
	"os/exec"
	"syscall"
)

// openURL 은 기본 브라우저로 url 을 연다.
func openURL(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
