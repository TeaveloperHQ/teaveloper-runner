package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TeaveloperHQ/teaveloper-runner/internal/config"
	"github.com/TeaveloperHQ/teaveloper-runner/internal/store"
)

// 소유자 스코프(개별 코드 열람)가 HTTP 레벨에서 격리를 강제하는지 — 핵심 보안 검증.
// "학생 A 는 B 의 레코드를 읽거나 지울 수 없다"를 실제 요청으로 증명한다.
func TestOwnerScopeHTTPIsolation(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def := `{"collections":{"results":{"write":true,"owner":{"field":"code","read":true,"edit":true}}}}`
	if err := os.WriteFile(filepath.Join(appDir, "teaveloper.json"), []byte(def), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "db.sqlite")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(&config.Config{}, st, appDir, dbPath)

	// 외부(공개) 요청 — dispatch 와 동일하게 withPublic 표식.
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req = req.WithContext(withPublic(req.Context()))
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, req)
		return rec
	}
	idOf := func(rec *httptest.ResponseRecorder) string {
		var m map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		return fmt.Sprint(m["id"])
	}

	// 1) A·B 제출(write 는 누구나)
	ra := do("POST", "/api/results", `{"code":"AAA","v":1}`)
	if ra.Code != 201 {
		t.Fatalf("A 제출=%d %s", ra.Code, ra.Body)
	}
	rb := do("POST", "/api/results", `{"code":"BBB","v":2}`)
	if rb.Code != 201 {
		t.Fatalf("B 제출=%d", rb.Code)
	}
	aID, bID := idOf(ra), idOf(rb)

	// 2) 코드 없이 목록 → 403 (전체 목록 불가)
	if r := do("GET", "/api/results", ""); r.Code != 403 {
		t.Errorf("코드 없는 목록=%d (403 기대)", r.Code)
	}
	// 3) A 코드 목록 → A 것만
	r := do("GET", "/api/results?code=AAA", "")
	if r.Code != 200 {
		t.Fatalf("A 목록=%d", r.Code)
	}
	var list []map[string]any
	_ = json.Unmarshal(r.Body.Bytes(), &list)
	if len(list) != 1 || fmt.Sprint(list[0]["code"]) != "AAA" {
		t.Errorf("A 목록 격리 실패: %v", list)
	}
	// 4) A 코드로 B 레코드 조회 → 404 (남의 것 숨김)
	if r := do("GET", "/api/results/"+bID+"?code=AAA", ""); r.Code != 404 {
		t.Errorf("A가 B 조회=%d (404 기대)", r.Code)
	}
	// 5) A 코드로 A 레코드 조회 → 200
	if r := do("GET", "/api/results/"+aID+"?code=AAA", ""); r.Code != 200 {
		t.Errorf("A가 A 조회=%d (200 기대)", r.Code)
	}
	// 6) A 코드로 B 레코드 삭제 시도 → 404, 그리고 B 는 살아있음
	if r := do("DELETE", "/api/results/"+bID+"?code=AAA", ""); r.Code != 404 {
		t.Errorf("A가 B 삭제=%d (404 기대)", r.Code)
	}
	if r := do("GET", "/api/results/"+bID+"?code=BBB", ""); r.Code != 200 {
		t.Errorf("B 레코드가 사라짐=%d (200 기대)", r.Code)
	}
}
