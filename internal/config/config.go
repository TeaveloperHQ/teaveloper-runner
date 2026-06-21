// Package config 는 exe 와 같은 폴더에 있는 config.json 을 읽고 검증한다.
// 포털("내 서버" → 터널 생성)에서 받은 설정 파일을 그대로 사용한다.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config 는 포털이 발급하는 config.json 의 형식이다.
//
//	{
//	  "gatewayUrl": "wss://gw.teaveloper.com/_agent",
//	  "slug": "class-abc",
//	  "publicUrl": "https://class-abc.teaveloper.com",
//	  "localPort": 8080,
//	  "token": "tnl_xxxxxxxxxxxxxxxxxxxx"
//	}
type Config struct {
	GatewayURL string `json:"gatewayUrl"`
	Slug       string `json:"slug"`
	PublicURL  string `json:"publicUrl"`
	LocalPort  int    `json:"localPort"`
	Token      string `json:"token"`

	baseDir string // config.json 을 찾은 폴더(= ./app/ 와 DB 의 기준)
}

// BaseDir 는 config.json 이 있던 폴더다. ./app/ 와 데이터 파일이 여기 기준으로 놓인다.
func (c *Config) BaseDir() string { return c.baseDir }

// AppDir 는 정적 프론트 폴더(baseDir/app)다.
func (c *Config) AppDir() string { return filepath.Join(c.baseDir, "app") }

// DBPath 는 SQLite 데이터 파일 경로다.
func (c *Config) DBPath() string { return filepath.Join(c.baseDir, "teaveloper-data.db") }

// 빌드 시 ldflags 로 주입할 수 있는 기본값. 포털이 교사별 exe 에 설정을 구워넣고
// 싶을 때 사용한다(선택). 비어 있으면 config.json 만 사용한다.
//
//	go build -ldflags "-X .../config.bakedJSON=$(cat config.json | base64)"
var bakedJSON string

// LocalBase 는 프록시 대상 로컬 주소다. 보안상 항상 127.0.0.1 로 고정한다.
// config 의 다른 필드로 호스트를 바꿀 수 없다(오픈 프록시 금지).
func (c *Config) LocalBase() string {
	return fmt.Sprintf("http://127.0.0.1:%d", c.LocalPort)
}

// ExeDir 는 실행 파일이 있는 디렉터리를 반환한다. config.json 을 여기서 찾는다.
func ExeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	// 심볼릭 링크 해소 (드물지만 안전하게)
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

// Load 는 exe 옆의 config.json 을 읽어 검증한다. 파일이 없고 ldflags 로 구워넣은
// 설정도 없으면 친절한 오류를 반환한다.
func Load() (*Config, error) {
	var raw []byte
	baseDir := "."

	dir, err := ExeDir()
	if err == nil {
		path := filepath.Join(dir, "config.json")
		if b, rerr := os.ReadFile(path); rerr == nil {
			raw = b
			baseDir = dir
		} else if !errors.Is(rerr, os.ErrNotExist) {
			return nil, fmt.Errorf("config.json 을 읽을 수 없습니다: %w", rerr)
		}
	}

	// 작업 디렉터리(개발 중 go run) 에서도 한 번 더 시도
	if raw == nil {
		if b, rerr := os.ReadFile("config.json"); rerr == nil {
			raw = b
			baseDir = "."
		}
	}

	if raw == nil && bakedJSON != "" {
		raw = []byte(bakedJSON)
		if dir != "" {
			baseDir = dir
		}
	}

	if raw == nil {
		return nil, errors.New("config.json 을 찾을 수 없습니다. 포털 '내 서버'에서 받은 config.json 을 이 프로그램과 같은 폴더에 두세요.")
	}

	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("config.json 형식이 올바르지 않습니다: %w", err)
	}
	c.baseDir = baseDir
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	c.GatewayURL = strings.TrimSpace(c.GatewayURL)
	c.Token = strings.TrimSpace(c.Token)
	c.Slug = strings.TrimSpace(c.Slug)
	c.PublicURL = strings.TrimSpace(c.PublicURL)

	if c.GatewayURL == "" {
		return errors.New("config.json 에 gatewayUrl 이 없습니다.")
	}
	u, err := url.Parse(c.GatewayURL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return fmt.Errorf("gatewayUrl 이 올바른 WebSocket 주소(ws:// 또는 wss://)가 아닙니다: %q", c.GatewayURL)
	}
	if c.Token == "" {
		return errors.New("config.json 에 token 이 없습니다.")
	}
	if c.LocalPort < 1 || c.LocalPort > 65535 {
		return fmt.Errorf("localPort 가 올바르지 않습니다(1~65535): %d", c.LocalPort)
	}
	return nil
}
