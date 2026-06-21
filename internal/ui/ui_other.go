//go:build !windows

// 비 Windows(개발/테스트용): 콘솔 모드. 트레이 대신 상태를 콘솔에 출력하고
// Ctrl+C 까지 실행한다. 코어(서버+터널) 로직을 리눅스에서 e2e 검증할 때 사용.
package ui

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/TeaveloperHQ/teacher-runner/internal/config"
	"github.com/TeaveloperHQ/teacher-runner/internal/server"
	"github.com/TeaveloperHQ/teacher-runner/internal/tunnel"
)

// Run 은 콘솔 모드로 터널 클라이언트를 구동한다.
func Run(cfg *config.Config, srv *server.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("공개 주소: %s", cfg.PublicURL)
	log.Printf("관리 페이지: %s", srv.AdminURL())

	client := tunnel.New(cfg, srv.TunnelHandler, func(s tunnel.Status) {
		srv.SetStatus(s)
		if s.Message != "" {
			log.Printf("[상태] %s — %s", s.State, s.Message)
		} else {
			log.Printf("[상태] %s", s.State)
		}
	})
	go client.Run(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Printf("종료합니다…")
}
