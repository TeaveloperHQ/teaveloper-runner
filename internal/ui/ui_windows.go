//go:build windows

// Windows: 시스템 트레이 아이콘 UI. 상태 표시 + 우클릭 메뉴(공개주소 열기/복사,
// 자동 실행 토글, 종료).
package ui

import (
	"context"
	"log"

	"github.com/TeaveloperHQ/teacher-runner/internal/autostart"
	"github.com/TeaveloperHQ/teacher-runner/internal/config"
	"github.com/TeaveloperHQ/teacher-runner/internal/server"
	"github.com/TeaveloperHQ/teacher-runner/internal/tunnel"
	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
)

// Run 은 트레이 UI 를 띄우고 터널 클라이언트를 구동한다. systray.Run 이
// 메인 스레드를 점유하므로 main 에서 직접 호출해야 한다.
func Run(cfg *config.Config, srv *server.Server) {
	a := &app{cfg: cfg, srv: srv}
	systray.Run(a.onReady, a.onExit)
}

type app struct {
	cfg    *config.Config
	srv    *server.Server
	cancel context.CancelFunc

	mStatus    *systray.MenuItem
	mURL       *systray.MenuItem
	mOpen      *systray.MenuItem
	mCopy      *systray.MenuItem
	mAdmin     *systray.MenuItem
	mAutostart *systray.MenuItem
	mQuit      *systray.MenuItem
}

func (a *app) onReady() {
	systray.SetIcon(iconConnecting)
	systray.SetTitle("")
	systray.SetTooltip("Teaveloper 에이전트 — 연결 중…")

	a.mStatus = systray.AddMenuItem("연결 중…", "")
	a.mStatus.Disable()
	a.mURL = systray.AddMenuItem(displayURL(a.cfg.PublicURL), a.cfg.PublicURL)
	a.mURL.Disable()

	systray.AddSeparator()
	a.mOpen = systray.AddMenuItem("🌐 공개 주소 열기", "기본 브라우저로 엽니다")
	a.mCopy = systray.AddMenuItem("📋 공개 주소 복사", "클립보드로 복사합니다")
	a.mAdmin = systray.AddMenuItem("🗂 관리 페이지 열기", "결과 열람·내보내기(내 컴퓨터 전용)")
	if a.cfg.PublicURL == "" {
		a.mOpen.Disable()
		a.mCopy.Disable()
	}

	systray.AddSeparator()
	a.mAutostart = systray.AddMenuItemCheckbox("Windows 시작 시 자동 실행", "로그온할 때 자동으로 켜집니다", autostart.Enabled())
	if !autostart.Supported() {
		a.mAutostart.Disable()
	}

	systray.AddSeparator()
	a.mQuit = systray.AddMenuItem("종료", "프로그램을 종료합니다")

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	client := tunnel.New(a.cfg, a.srv.TunnelHandler, a.onStatus)
	go client.Run(ctx)
	go a.handleClicks()
}

func (a *app) onExit() {
	if a.cancel != nil {
		a.cancel()
	}
}

// onStatus 는 터널 고루틴에서 호출된다. systray 의 Set* 는 스레드 안전하다.
func (a *app) onStatus(s tunnel.Status) {
	a.srv.SetStatus(s) // 관리 페이지가 상태를 읽을 수 있게 저장
	switch s.State {
	case tunnel.StateConnecting:
		systray.SetIcon(iconConnecting)
		a.mStatus.SetTitle("연결 중…")
		systray.SetTooltip("Teaveloper — 연결 중…")
	case tunnel.StateConnected:
		systray.SetIcon(iconConnected)
		a.mStatus.SetTitle("🟢 연결됨")
		systray.SetTooltip("Teaveloper — 연결됨\n" + a.cfg.PublicURL)
	case tunnel.StateDisconnected:
		systray.SetIcon(iconDisconnected)
		a.mStatus.SetTitle("🔴 끊김 — 재연결 중…")
		systray.SetTooltip("Teaveloper — 끊김, 재연결 시도 중")
	case tunnel.StateForbidden:
		systray.SetIcon(iconDisconnected)
		a.mStatus.SetTitle("⚠ 토큰 무효 — 설정 다시 받기")
		systray.SetTooltip("Teaveloper — " + s.Message)
	case tunnel.StateLocalDown:
		systray.SetIcon(iconLocalDown)
		a.mStatus.SetTitle("🟠 로컬 앱 미응답")
		systray.SetTooltip("Teaveloper — " + s.Message)
	}
}

func (a *app) handleClicks() {
	for {
		select {
		case <-a.mOpen.ClickedCh:
			if err := openURL(a.cfg.PublicURL); err != nil {
				log.Printf("주소 열기 실패: %v", err)
			}
		case <-a.mCopy.ClickedCh:
			if err := clipboard.WriteAll(a.cfg.PublicURL); err != nil {
				log.Printf("복사 실패: %v", err)
			}
		case <-a.mAdmin.ClickedCh:
			if err := openURL(a.srv.AdminURL()); err != nil {
				log.Printf("관리 페이지 열기 실패: %v", err)
			}
		case <-a.mAutostart.ClickedCh:
			a.toggleAutostart()
		case <-a.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (a *app) toggleAutostart() {
	if a.mAutostart.Checked() {
		if err := autostart.Disable(); err != nil {
			log.Printf("자동시작 해제 실패: %v", err)
			return
		}
		a.mAutostart.Uncheck()
	} else {
		if err := autostart.Enable(); err != nil {
			log.Printf("자동시작 등록 실패: %v", err)
			return
		}
		a.mAutostart.Check()
	}
}

// displayURL 은 메뉴에 보기 좋게 줄인 주소 텍스트를 만든다.
func displayURL(u string) string {
	if u == "" {
		return "(공개 주소 없음)"
	}
	return u
}
