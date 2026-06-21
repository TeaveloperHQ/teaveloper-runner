// teacher-runner — 교사 PC 러너.
//
// 한 exe 안에서: ① ./app/ 정적 프론트 서빙 ② 내장 데이터 API(/api, 프리셋 권한)
// ③ 소유자 관리 페이지(/_admin, 로컬 전용) ④ 게이트웨이 터널 클라이언트.
// 데이터는 교사 PC 의 SQLite 에만 저장된다(게이트웨이는 무저장 중계).
//
// 빌드(포털/CI, 콘솔 창 없이, CGO 불필요):
//
//	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
//	  go build -trimpath -ldflags "-H windowsgui -s -w" -o teaveloper-runner.exe .
package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/TeaveloperHQ/teacher-runner/internal/config"
	"github.com/TeaveloperHQ/teacher-runner/internal/server"
	"github.com/TeaveloperHQ/teacher-runner/internal/store"
	"github.com/TeaveloperHQ/teacher-runner/internal/ui"
)

func main() {
	setupLogging()

	cfg, err := config.Load()
	if err != nil {
		log.Printf("설정 로드 실패: %v", err)
		ui.FatalError("Teaveloper 러너 — 설정 오류", err.Error())
		os.Exit(1)
	}

	st, err := store.Open(cfg.DBPath())
	if err != nil {
		log.Printf("데이터 저장소 열기 실패: %v", err)
		ui.FatalError("Teaveloper 러너 — 저장소 오류",
			"데이터 파일을 열 수 없습니다:\n"+err.Error())
		os.Exit(1)
	}
	defer st.Close()

	srv := server.New(cfg, st, cfg.AppDir(), cfg.DBPath())

	// 로컬(소유자) 리스너 — 반드시 127.0.0.1 에만 바인딩(LAN/외부 접근 차단).
	addr := "127.0.0.1:" + strconv.Itoa(cfg.LocalPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("로컬 포트 열기 실패(%s): %v", addr, err)
		ui.FatalError("Teaveloper 러너 — 포트 사용 중",
			"포트 "+strconv.Itoa(cfg.LocalPort)+" 을(를) 열 수 없습니다. 다른 프로그램이 쓰고 있는지 확인하세요.\n"+err.Error())
		os.Exit(1)
	}
	go func() {
		log.Printf("로컬 서버: http://%s  (관리: %s)", addr, srv.AdminURL())
		if err := http.Serve(ln, srv.OwnerHandler()); err != nil {
			log.Printf("로컬 서버 종료: %v", err)
		}
	}()

	log.Printf("시작: slug=%s public=%s app=%s", cfg.Slug, cfg.PublicURL, cfg.AppDir())
	ui.Run(cfg, srv) // 트레이(Windows) 또는 콘솔(개발). 종료까지 블로킹.
}

// setupLogging 은 로그를 baseDir 의 teaveloper-runner.log 에 남긴다. GUI 모드
// (-H windowsgui)에서는 콘솔이 없어 화면 출력이 안 보이므로 파일 로그가 진단의 핵심.
func setupLogging() {
	log.SetFlags(log.LstdFlags)

	dir, err := config.ExeDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "teaveloper-runner.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	if isConsole() {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	} else {
		log.SetOutput(f)
	}
}
