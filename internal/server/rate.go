package server

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// ipLimiter 는 IP 별 토큰버킷 레이트리미터다. 공개(외부) 요청에만 적용한다.
type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
	rps     rate.Limit
	burst   int
}

func newIPLimiter(rps, burst int) *ipLimiter {
	return &ipLimiter{
		buckets: map[string]*rate.Limiter{},
		rps:     rate.Limit(rps),
		burst:   burst,
	}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	lim, ok := l.buckets[ip]
	if !ok {
		// 메모리 폭주 방지: 버킷이 너무 많아지면 초기화(거친 안전장치).
		if len(l.buckets) > 10000 {
			l.buckets = map[string]*rate.Limiter{}
		}
		lim = rate.NewLimiter(l.rps, l.burst)
		l.buckets[ip] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

// clientIP 는 방문자 IP 를 추정한다. 터널 경유 시 게이트웨이/Caddy 가 넣은
// X-Forwarded-For 의 첫 IP 를 쓰고, 없으면 RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-Ip"); xr != "" {
		return strings.TrimSpace(xr)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
