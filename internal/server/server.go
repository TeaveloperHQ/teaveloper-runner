// Package server 는 러너의 로컬 HTTP 서버다. 정적 프론트(./app/) 서빙 +
// 내장 데이터 API(/api, 프리셋 권한 강제) + 소유자 관리 페이지(/_admin, 로컬 전용).
//
// 보안 핵심: 요청이 "공개(터널 경유)"인지 "로컬(소유자)"인지는 context 표식으로
// 100% 확정한다(헤더 추론 아님). 터널 경유 요청만 publicKey 가 박힌다.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/TeaveloperHQ/teaveloper-runner/internal/appdef"
	"github.com/TeaveloperHQ/teaveloper-runner/internal/config"
	"github.com/TeaveloperHQ/teaveloper-runner/internal/store"
	"github.com/TeaveloperHQ/teaveloper-runner/internal/tunnel"
)

// 어뷰즈 방지 상한.
const (
	maxBodyBytes = 100 << 10 // 레코드 본문 100KB
	maxRecords   = 50000     // 컬렉션당 레코드 수 상한
	publicRPS    = 5         // 공개 IP 당 초당 요청
	publicBurst  = 20        // 공개 IP 당 버스트
)

type ctxKey int

const publicKey ctxKey = 0

// withPublic 은 요청을 공개(외부 방문자) 요청으로 표시한다. 터널 dispatch 에서만 호출.
func withPublic(ctx context.Context) context.Context {
	return context.WithValue(ctx, publicKey, true)
}

// isPublic 은 요청이 터널(외부)로 들어왔는지 본다. 표식이 없으면 로컬(소유자).
func isPublic(r *http.Request) bool {
	v, _ := r.Context().Value(publicKey).(bool)
	return v
}

// Server 는 러너의 로컬 HTTP 서버 상태를 담는다.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	appDir string
	dbPath string
	mux    *http.ServeMux

	defMu      sync.RWMutex
	def        *appdef.Def
	defModTime int64

	statusMu sync.RWMutex
	status   tunnel.Status

	limiter *ipLimiter
}

// New 는 서버를 만든다. appDir 은 정적 프론트(./app/)의 경로다.
func New(cfg *config.Config, st *store.Store, appDir, dbPath string) *Server {
	s := &Server{
		cfg:     cfg,
		store:   st,
		appDir:  appDir,
		dbPath:  dbPath,
		limiter: newIPLimiter(publicRPS, publicBurst),
	}
	s.loadDef() // 시작 시 1회(이후 mtime 변경 시 자동 재로딩)

	mux := http.NewServeMux()
	// 데이터 API (프리셋 권한 강제 + CORS — 외부 프론트 교차출처 지원)
	mux.HandleFunc("POST /api/{collection}", withCORS(s.handleCreate))
	mux.HandleFunc("GET /api/{collection}", withCORS(s.handleList))
	mux.HandleFunc("GET /api/{collection}/{id}", withCORS(s.handleGet))
	mux.HandleFunc("PATCH /api/{collection}/{id}", withCORS(s.handlePatch))
	mux.HandleFunc("DELETE /api/{collection}/{id}", withCORS(s.handleDelete))
	// CORS 프리플라이트 (프리셋/본문 처리 없이 204)
	mux.HandleFunc("OPTIONS /api/{collection}", s.handlePreflight)
	mux.HandleFunc("OPTIONS /api/{collection}/{id}", s.handlePreflight)
	// 소유자 관리 페이지 (로컬 전용 — 공개 요청은 adminGuard 가 404 처리)
	mux.HandleFunc("/_admin", s.handleAdminPage)
	mux.HandleFunc("/_admin/", s.handleAdminPage)
	mux.HandleFunc("GET /_admin/api/status", s.adminGuard(s.adminStatus))
	mux.HandleFunc("GET /_admin/api/collections", s.adminGuard(s.adminCollections))
	mux.HandleFunc("GET /_admin/api/records/{collection}", s.adminGuard(s.adminList))
	mux.HandleFunc("DELETE /_admin/api/records/{collection}", s.adminGuard(s.adminDeleteAll))
	mux.HandleFunc("DELETE /_admin/api/records/{collection}/{id}", s.adminGuard(s.adminDelete))
	mux.HandleFunc("GET /_admin/api/export/{collection}", s.adminGuard(s.adminExport))
	// 정적 프론트
	mux.HandleFunc("/", s.handleStatic)
	s.mux = mux
	return s
}

// OwnerHandler 는 loopback(127.0.0.1) 리스너용 핸들러다. 표식이 없으므로 로컬/소유자.
func (s *Server) OwnerHandler() http.Handler { return s.mux }

// SetStatus 는 터널 상태를 저장한다(관리 페이지 표시용).
func (s *Server) SetStatus(st tunnel.Status) {
	s.statusMu.Lock()
	s.status = st
	s.statusMu.Unlock()
}

func (s *Server) getStatus() tunnel.Status {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

// AdminURL 은 로컬 관리 페이지 주소다.
func (s *Server) AdminURL() string {
	return "http://localhost:" + strconv.Itoa(s.cfg.LocalPort) + "/_admin"
}

// ── 데이터 API ────────────────────────────────────────────

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	if ok, _ := s.gate(w, r, col, http.MethodPost); !ok {
		return
	}
	if n, _ := s.store.Count(col); n >= maxRecords {
		writeErr(w, http.StatusTooManyRequests, "이 컬렉션이 가득 찼습니다(최대 "+strconv.Itoa(maxRecords)+"개).")
		return
	}
	body, err := readBody(w, r)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "본문이 너무 큽니다(최대 100KB).")
		return
	}
	rec, err := s.store.Insert(col, body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	ok, sc := s.gate(w, r, col, http.MethodGet)
	if !ok {
		return
	}
	q := r.URL.Query()
	opts := store.ListOpts{
		Sort:    q.Get("sort"),
		Desc:    q.Get("order") == "desc",
		Filters: map[string]string{},
	}
	if l := q.Get("limit"); l != "" {
		opts.Limit, _ = strconv.Atoi(l)
	}
	for k, v := range q {
		switch k {
		case "sort", "order", "limit":
			continue
		default:
			if len(v) > 0 {
				opts.Filters[k] = v[0]
			}
		}
	}
	if sc != nil {
		opts.Filters[sc.field] = sc.code // 소유자 스코프: 자기 코드 레코드만
	}
	list, err := s.store.List(col, opts)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	ok, sc := s.gate(w, r, col, http.MethodGet)
	if !ok {
		return
	}
	rec, err := s.store.Get(col, r.PathValue("id"))
	if err == store.ErrNotFound {
		writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ownerMatch(rec, sc) {
		writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.") // 남의 것은 존재 자체를 숨김
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handlePatch(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	ok, sc := s.gate(w, r, col, http.MethodPatch)
	if !ok {
		return
	}
	if sc != nil { // 소유자 스코프: 자기 코드 레코드만 수정
		cur, gerr := s.store.Get(col, r.PathValue("id"))
		if gerr != nil || !ownerMatch(cur, sc) {
			writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.")
			return
		}
	}
	body, err := readBody(w, r)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "본문이 너무 큽니다(최대 100KB).")
		return
	}
	rec, err := s.store.Patch(col, r.PathValue("id"), body)
	if err == store.ErrNotFound {
		writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	ok, sc := s.gate(w, r, col, http.MethodDelete)
	if !ok {
		return
	}
	if sc != nil { // 소유자 스코프: 자기 코드 레코드만 삭제
		cur, gerr := s.store.Get(col, r.PathValue("id"))
		if gerr != nil || !ownerMatch(cur, sc) {
			writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.")
			return
		}
	}
	err := s.store.Delete(col, r.PathValue("id"))
	if err == store.ErrNotFound {
		writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ownerScope 는 "소유자 코드로 스코프된" 접근을 나타낸다. gate 가 반환하면 핸들러는
// 해당 코드와 일치하는 레코드에만 동작해야 한다.
type ownerScope struct {
	field string
	code  string
}

// ownerMatch 는 레코드가 스코프 코드의 소유인지 본다(필드 값 == 코드, 문자열 비교).
func ownerMatch(rec map[string]any, sc *ownerScope) bool {
	if sc == nil {
		return true
	}
	v, ok := rec[sc.field]
	if !ok {
		return false
	}
	return fmt.Sprintf("%v", v) == sc.code
}

// gate 는 컬렉션 선언 여부 + 공개 요청 권한 + 레이트리밋을 강제한다.
// 통과하면 (true, sc). sc != nil 이면 핸들러가 그 코드로 레코드를 스코프해야 한다.
func (s *Server) gate(w http.ResponseWriter, r *http.Request, collection, method string) (bool, *ownerScope) {
	def := s.currentDef()
	if def == nil {
		writeErr(w, http.StatusServiceUnavailable, "앱이 아직 로드되지 않았습니다. ./app/teaveloper.json 을 넣고 다시 시도하세요.")
		return false, nil
	}
	perm, declared := def.Perm(collection)
	if !declared {
		writeErr(w, http.StatusNotFound, "선언되지 않은 컬렉션입니다: "+collection+" (teaveloper.json 의 collections 에 추가하세요)")
		return false, nil
	}
	if !isPublic(r) {
		return true, nil // 로컬 소유자 = 전체 권한(스코프 없음)
	}
	// 공개 요청에만 레이트리밋 적용(로컬 소유자는 신뢰).
	if !s.limiter.allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "요청이 너무 많습니다. 잠시 후 다시 시도하세요.")
		return false, nil
	}
	// 1) 비스코프(누구나) 권한이 있으면 전체 허용.
	if perm.AllowsPublic(method) {
		return true, nil
	}
	// 2) 소유자 스코프 — ?{field}=코드 가 있어야 하고, 그 코드 레코드에만.
	if perm.Owner.Allows(method) {
		code := r.URL.Query().Get(perm.Owner.Field)
		if code == "" {
			writeErr(w, http.StatusForbidden, "본인 코드가 필요합니다(?"+perm.Owner.Field+"=... 형식).")
			return false, nil
		}
		return true, &ownerScope{field: perm.Owner.Field, code: code}
	}
	writeErr(w, http.StatusForbidden, "이 작업은 외부 방문자에게 허용되지 않습니다(권한 제한).")
	return false, nil
}

// ── 정적 프론트 ───────────────────────────────────────────

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "허용되지 않은 메서드")
		return
	}
	// 경로 정규화 + 탈출 방지
	rel := filepath.Clean("/" + r.URL.Path) // 항상 / 로 시작, .. 제거
	full := filepath.Join(s.appDir, filepath.FromSlash(rel))
	if s.appDir == "" || !withinDir(s.appDir, full) {
		s.servePlaceholder(w, r)
		return
	}

	info, err := os.Stat(full)
	if err == nil && info.IsDir() {
		full = filepath.Join(full, "index.html")
		info, err = os.Stat(full)
	}
	if err != nil || info.IsDir() {
		// 파일 없음 → SPA 폴백(index.html) 또는 안내 페이지
		index := filepath.Join(s.appDir, "index.html")
		if _, e := os.Stat(index); e == nil {
			http.ServeFile(w, r, index)
			return
		}
		s.servePlaceholder(w, r)
		return
	}
	http.ServeFile(w, r, full)
}

// servePlaceholder 는 앱 파일이 없을 때 안내 페이지를 보여준다. 로컬(소유자)에는
// "여기에 앱 파일을 넣으세요" + 폴더 경로, 공개 방문자에는 "준비 중".
func (s *Server) servePlaceholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if isPublic(r) {
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>준비 중</title>` +
			`<body style="font-family:system-ui;max-width:32rem;margin:4rem auto;text-align:center">` +
			`<h1>🛠️ 준비 중</h1><p>이 페이지는 아직 준비 중입니다.</p></body>`))
		return
	}
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>앱 파일을 넣으세요</title>` +
		`<body style="font-family:system-ui;max-width:40rem;margin:3rem auto;line-height:1.7">` +
		`<h1>📂 앱 파일을 넣어 주세요</h1>` +
		`<p>AI가 만들어 준 앱 파일(<code>index.html</code> 등)과 <code>teaveloper.json</code> 을 아래 폴더에 저장하세요:</p>` +
		`<pre style="background:#f4f4f5;padding:1rem;border-radius:8px">` + htmlEscape(s.appDir) + `</pre>` +
		`<p>관리 페이지: <a href="/_admin">/_admin</a></p></body>`))
}

// ── 응답/유틸 ─────────────────────────────────────────────

func readBody(w http.ResponseWriter, r *http.Request) (json.RawMessage, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var raw json.RawMessage
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func withinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel == "." || (len(rel) < 2 || rel[:2] != "..")
}

func htmlEscape(s string) string {
	r := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		switch c {
		case '<':
			r = append(r, "&lt;"...)
		case '>':
			r = append(r, "&gt;"...)
		case '&':
			r = append(r, "&amp;"...)
		default:
			r = append(r, c)
		}
	}
	return string(r)
}
