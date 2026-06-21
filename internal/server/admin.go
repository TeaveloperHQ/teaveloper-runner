package server

import (
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/TeaveloperHQ/teacher-runner/internal/store"
)

//go:embed admin.html
var adminHTML []byte

// adminGuard 는 공개(터널) 요청을 404 로 막는다. 관리 API 는 로컬 전용.
func (s *Server) adminGuard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r) {
			http.NotFound(w, r) // 존재 자체를 숨긴다
			return
		}
		h(w, r)
	}
}

// handleAdminPage 는 관리 페이지 HTML 을 서빙한다(로컬 전용).
func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	if isPublic(r) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(adminHTML)
}

// adminStatus 는 연결 상태 + 공개 주소를 반환한다.
func (s *Server) adminStatus(w http.ResponseWriter, r *http.Request) {
	st := s.getStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"state":     st.State.String(),
		"message":   st.Message,
		"publicUrl": s.cfg.PublicURL,
		"slug":      s.cfg.Slug,
		"localPort": s.cfg.LocalPort,
	})
}

// adminCollections 는 선언된 컬렉션 목록 + 프리셋 + 레코드 수를 반환한다.
func (s *Server) adminCollections(w http.ResponseWriter, r *http.Request) {
	def := s.currentDef()
	out := []map[string]any{}
	if def != nil {
		names := make([]string, 0, len(def.Collections))
		for name := range def.Collections {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			n, _ := s.store.Count(name)
			out = append(out, map[string]any{
				"name":   name,
				"preset": string(def.Collections[name]),
				"count":  n,
			})
		}
	}
	appName := ""
	if def != nil {
		appName = def.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": appName, "collections": out})
}

// adminList 는 컬렉션 레코드를 반환한다(로컬 — 프리셋 무관 전체 열람).
func (s *Server) adminList(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	if !s.declared(col) {
		writeErr(w, http.StatusNotFound, "선언되지 않은 컬렉션: "+col)
		return
	}
	opts := store.ListOpts{Desc: true} // 최신순
	if l := r.URL.Query().Get("limit"); l != "" {
		opts.Limit, _ = strconv.Atoi(l)
	}
	list, err := s.store.List(col, opts)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) adminDelete(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	if err := s.store.Delete(col, r.PathValue("id")); err == store.ErrNotFound {
		writeErr(w, http.StatusNotFound, "레코드를 찾을 수 없습니다.")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) adminDeleteAll(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	n, err := s.store.DeleteAll(col)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
}

// adminExport 는 컬렉션을 CSV 또는 JSON 으로 내보낸다(?format=csv|json, 기본 csv).
func (s *Server) adminExport(w http.ResponseWriter, r *http.Request) {
	col := r.PathValue("collection")
	if !s.declared(col) {
		writeErr(w, http.StatusNotFound, "선언되지 않은 컬렉션: "+col)
		return
	}
	records, err := s.store.List(col, store.ListOpts{Desc: false, Limit: maxRecords})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+col+".json\"")
		_ = json.NewEncoder(w).Encode(records)
		return
	}

	// CSV: id,createdAt,updatedAt 를 앞에 두고 나머지 키를 정렬해 헤더로.
	cols := csvColumns(records)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+col+".csv\"")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM (엑셀 한글 깨짐 방지)
	cw := csv.NewWriter(w)
	_ = cw.Write(cols)
	for _, rec := range records {
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = csvCell(rec[c])
		}
		_ = cw.Write(row)
	}
	cw.Flush()
}

// ── helpers ──

func (s *Server) declared(col string) bool {
	def := s.currentDef()
	return def != nil && def.Declared(col)
}

var reservedFirst = []string{"id", "createdAt", "updatedAt"}

func csvColumns(records []map[string]any) []string {
	seen := map[string]bool{"id": true, "createdAt": true, "updatedAt": true}
	rest := []string{}
	for _, rec := range records {
		for k := range rec {
			if !seen[k] {
				seen[k] = true
				rest = append(rest, k)
			}
		}
	}
	sort.Strings(rest)
	return append(append([]string{}, reservedFirst...), rest...)
}

func csvCell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
