// Package tunnel 은 게이트웨이에 붙어 들어오는 공개 HTTP 요청을 러너 자신의
// 로컬 핸들러로 중계하는 전송 계층이다. 단일 WebSocket 위에서 다수 요청을 id 로
// 다중화한다. 요청을 어떻게 처리할지는 주입된 RequestHandler 가 결정한다.
package tunnel

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/TeaveloperHQ/teacher-runner/internal/config"
	"github.com/gorilla/websocket"
)

const (
	// 재연결 백오프
	backoffMin = 1 * time.Second
	backoffMax = 30 * time.Second

	// 하트비트: 게이트웨이가 30초마다 ping 을 보낸다. 그보다 넉넉히 잡고,
	// ping 을 받을 때마다 read deadline 을 갱신한다(죽은 연결 감지).
	readWait  = 70 * time.Second
	writeWait = 10 * time.Second

	handshakeTimeout = 15 * time.Second
)

// Client 는 하나의 터널 연결을 관리한다(자동 재연결 포함).
type Client struct {
	cfg      *config.Config
	handler  RequestHandler
	notifier *stateNotifier
}

// New 는 터널 클라이언트를 만든다. handler 는 각 req 프레임을 처리하고, onStatus 는
// 상태가 바뀔 때마다 호출된다(UI 갱신용).
func New(cfg *config.Config, handler RequestHandler, onStatus func(Status)) *Client {
	return &Client{
		cfg:      cfg,
		handler:  handler,
		notifier: &stateNotifier{cb: onStatus},
	}
}

// Run 은 ctx 가 취소될 때까지 연결을 유지한다. 끊기면 지수 백오프로 재연결한다.
// 토큰 무효(403)면 재시도하지 않고 StateForbidden 으로 멈춘다.
func (c *Client) Run(ctx context.Context) {
	backoff := backoffMin
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.notifier.set(Status{State: StateConnecting})
		connected, err := c.connectOnce(ctx)

		if ctx.Err() != nil {
			return
		}

		// 한 번이라도 성립했다 끊긴 거면 백오프 리셋.
		if connected {
			backoff = backoffMin
		}

		// 토큰 무효 → 재시도 의미 없음
		if errors.Is(err, errForbidden) {
			c.notifier.set(Status{
				State:   StateForbidden,
				Message: "토큰이 더 이상 유효하지 않습니다. 포털 '내 서버'에서 설정(config.json)을 다시 받으세요.",
			})
			return
		}

		c.notifier.set(Status{State: StateDisconnected, Message: backoffMsg(backoff)})
		log.Printf("연결 끊김: %v — %s 후 재연결", err, backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(backoff)):
		}
		backoff *= 2
		if backoff > backoffMax {
			backoff = backoffMax
		}
	}
}

var errForbidden = errors.New("forbidden")

// connectOnce 는 한 번 연결해 끊길 때까지 읽기 루프를 돈다.
// 첫 반환값은 dial 성공해 연결이 성립했는지다(백오프 리셋 판단용).
func (c *Client) connectOnce(ctx context.Context) (bool, error) {
	dialer := websocket.Dialer{HandshakeTimeout: handshakeTimeout}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.cfg.Token)

	conn, resp, err := dialer.DialContext(ctx, c.cfg.GatewayURL, h)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			return false, errForbidden
		}
		return false, err
	}
	defer conn.Close()

	c.notifier.set(Status{State: StateConnected, Message: c.cfg.PublicURL})
	log.Printf("게이트웨이 연결됨: %s (공개 %s)", c.cfg.GatewayURL, c.cfg.PublicURL)

	// read deadline + ping 수신 시 갱신. gorilla 기본은 ping 에 자동 pong.
	_ = conn.SetReadDeadline(time.Now().Add(readWait))
	conn.SetPingHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(readWait))
		err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}
		return err
	})

	// gorilla 는 동시 writer 를 금지한다 → 모든 쓰기를 이 mutex 로 직렬화한다.
	// 요청 처리는 고루틴 병렬, 응답 쓰기만 직렬화. (참조 구현에서 실제로 터졌던 버그)
	var writeMu sync.Mutex
	writeFrame := func(f Frame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		return conn.WriteJSON(f)
	}

	// ctx 취소 시 연결을 닫아 ReadJSON 을 깨운다.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	for {
		var f Frame
		if err := conn.ReadJSON(&f); err != nil {
			return true, err
		}
		if f.Type != "req" {
			continue
		}
		go func(req Frame) {
			res := c.handler(req)
			if err := writeFrame(res); err != nil {
				log.Printf("응답 전송 실패 id=%s: %v", req.ID, err)
			}
		}(f)
	}
}

func backoffMsg(d time.Duration) string {
	return "재연결 대기 " + d.String()
}

// jitter 는 백오프에 ±20% 무작위를 더해 동시 재연결 쏠림을 줄인다.
func jitter(d time.Duration) time.Duration {
	delta := float64(d) * 0.2
	return d + time.Duration((rand.Float64()*2-1)*delta)
}
