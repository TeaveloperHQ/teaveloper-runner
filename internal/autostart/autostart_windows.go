// Package autostart 는 Windows 작업 스케줄러(로그온 트리거)로 자동시작을 토글한다.
// 관리자 권한이 필요 없도록 /RL LIMITED 로 등록한다.
package autostart

import (
	"os"
	"os/exec"
	"syscall"
)

const taskName = "TeaveloperAgent"

// hideWindow 는 schtasks 호출 시 콘솔 창이 깜빡이지 않게 한다.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// Enabled 는 자동시작 작업이 등록돼 있는지 확인한다.
func Enabled() bool {
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName)
	hideWindow(cmd)
	return cmd.Run() == nil
}

// Enable 은 현재 exe 를 로그온 시 자동 실행하도록 등록한다.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("schtasks",
		"/Create", "/F",
		"/SC", "ONLOGON",
		"/RL", "LIMITED",
		"/TN", taskName,
		"/TR", `"`+exe+`"`,
	)
	hideWindow(cmd)
	return cmd.Run()
}

// Disable 은 자동시작 등록을 해제한다.
func Disable() error {
	cmd := exec.Command("schtasks", "/Delete", "/F", "/TN", taskName)
	hideWindow(cmd)
	return cmd.Run()
}

// Supported 는 이 플랫폼에서 자동시작 토글을 지원하는지 알린다.
func Supported() bool { return true }
