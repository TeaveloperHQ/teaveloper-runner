package server

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http/httptest"
	"strings"

	"github.com/TeaveloperHQ/teacher-runner/internal/tunnel"
)

// TunnelHandler 는 게이트웨이가 보낸 req 프레임을 러너의 mux 로 in-process 처리한다.
// ★ 보안 핵심: 여기서 withPublic 로 "공개 요청" 표식을 박는다. loopback 리스너는
// 절대 이 표식을 안 박으므로, 공개/로컬 구분이 코드 경로로 확정된다(헤더 추론 아님).
// 따라서 외부 방문자는 /_admin 에 물리적으로 도달할 수 없고, 프리셋 게이트가 강제된다.
func (s *Server) TunnelHandler(req tunnel.Frame) tunnel.Frame {
	body, err := base64.StdEncoding.DecodeString(req.Body)
	if err != nil {
		return tunnel.Frame{Type: "err", ID: req.ID, Error: "본문 디코딩 실패: " + err.Error()}
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}
	hr := httptest.NewRequest(method, req.Path, bytes.NewReader(body))
	for k, v := range req.Headers {
		// Host 는 게이트웨이가 공개 도메인으로 채워 보낸다. 라우팅은 경로 기반이라
		// 불필요하고, 혹시 모를 혼선을 피하려 제외한다.
		if strings.EqualFold(k, "Host") {
			continue
		}
		hr.Header.Set(k, v)
	}
	hr = hr.WithContext(withPublic(hr.Context()))

	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, hr)

	resp := rec.Result()
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[k] = strings.Join(v, ", ")
	}
	return tunnel.Frame{
		Type:    "res",
		ID:      req.ID,
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    base64.StdEncoding.EncodeToString(rb),
	}
}
