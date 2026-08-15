package tunnel

import "sync"

// State 는 사용자에게 보여줄 연결 상태다.
type State int

const (
	StateConnecting   State = iota // 연결 시도 중
	StateConnected                 // 연결됨 — 정상
	StateDisconnected              // 끊김 — 재연결 시도 중
	StateForbidden                 // 토큰 무효(403) — 재시도 안 함
	StateLocalDown                 // 게이트웨이엔 붙었으나 로컬 앱이 응답 없음
)

// String 은 화면에 그대로 쓰는 한국어다. 표현은 언제든 다듬을 수 있으므로
// 프로그램이 이 값을 비교해서는 안 된다 — 그 용도는 Code 다.
func (s State) String() string {
	switch s {
	case StateConnecting:
		return "연결 중"
	case StateConnected:
		return "연결됨"
	case StateDisconnected:
		return "끊김"
	case StateForbidden:
		return "토큰 무효"
	case StateLocalDown:
		return "로컬 앱 미응답"
	default:
		return "알 수 없음"
	}
}

// Code 는 기계가 읽는 안정적인 상태 코드다. /_admin/api/status 로 나가고 외부
// 도구(Teavel 등)가 연결 여부를 판단하는 근거이므로 **바꾸지 않는다**.
// 새 상태가 생기면 여기에 코드를 더하되, 기존 값의 철자는 그대로 둔다.
func (s State) Code() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateDisconnected:
		return "disconnected"
	case StateForbidden:
		return "forbidden"
	case StateLocalDown:
		return "localdown"
	default:
		return "unknown"
	}
}

// Status 는 UI 로 전달되는 상태 스냅샷이다.
type Status struct {
	State   State
	Message string // 사람용 부가 설명(있으면)
}

// stateNotifier 는 상태 변경을 UI 콜백으로 흘려보낸다. 콜백은 임의 고루틴에서
// 호출될 수 있으므로 UI 쪽에서 스레드 안전하게 처리해야 한다.
type stateNotifier struct {
	mu   sync.Mutex
	last Status
	cb   func(Status)
}

func (n *stateNotifier) set(s Status) {
	n.mu.Lock()
	changed := s != n.last
	n.last = s
	cb := n.cb
	n.mu.Unlock()
	if changed && cb != nil {
		cb(s)
	}
}
