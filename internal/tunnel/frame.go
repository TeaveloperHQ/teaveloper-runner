package tunnel

// Frame 은 게이트웨이 ↔ 러너 프레이밍 프로토콜이다. 게이트웨이 측 정의와 정확히
// 동일해야 한다(본문은 base64). 단일 WebSocket 위에서 여러 요청을 id 로 다중화.
type Frame struct {
	Type    string            `json:"type"` // "req" | "res" | "err"
	ID      string            `json:"id"`
	Method  string            `json:"method,omitempty"`
	Path    string            `json:"path,omitempty"`
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// RequestHandler 는 게이트웨이가 보낸 req 프레임을 처리해 res(또는 err) 프레임을
// 돌려준다. 러너는 여기서 자신의 로컬 HTTP 핸들러를 in-process 로 호출한다
// (공개 요청 표식을 코드 경로로 박아 헤더 스푸핑 표면을 없앤다).
type RequestHandler func(Frame) Frame
