package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestCRUD(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Insert
	r, err := s.Insert("responses", json.RawMessage(`{"name":"홍길동","score":3}`))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := r["id"].(string)
	if id == "" {
		t.Fatal("id 미부여")
	}
	if r["name"] != "홍길동" {
		t.Fatalf("name 보존 실패: %v", r["name"])
	}
	if _, ok := r["createdAt"]; !ok {
		t.Fatal("createdAt 미부여")
	}

	// Get
	g, err := s.Get("responses", id)
	if err != nil {
		t.Fatal(err)
	}
	if g["name"] != "홍길동" {
		t.Fatalf("get name: %v", g["name"])
	}

	// Second insert + List sorted desc
	_, _ = s.Insert("responses", json.RawMessage(`{"name":"이순신","score":5}`))
	list, err := s.List("responses", ListOpts{Desc: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list len=%d", len(list))
	}
	if list[0]["name"] != "이순신" {
		t.Fatalf("desc sort 실패: %v", list[0]["name"])
	}

	// Filter by data field
	f, err := s.List("responses", ListOpts{Filters: map[string]string{"name": "홍길동"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0]["name"] != "홍길동" {
		t.Fatalf("filter 실패: %+v", f)
	}

	// Patch (reserved keys ignored)
	p, err := s.Patch("responses", id, json.RawMessage(`{"score":9,"id":"hacked"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p["id"] == "hacked" {
		t.Fatal("예약 키 id 가 덮어써짐")
	}
	// score 는 json.Number 로 보존되므로 문자열 비교
	if jn, ok := p["score"].(json.Number); !ok || jn.String() != "9" {
		t.Fatalf("patch score: %v (%T)", p["score"], p["score"])
	}

	// Count
	if n, _ := s.Count("responses"); n != 2 {
		t.Fatalf("count=%d", n)
	}

	// Delete
	if err := s.Delete("responses", id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("responses", id); err != ErrNotFound {
		t.Fatalf("삭제 후 조회 err=%v", err)
	}

	// Non-object body rejected
	if _, err := s.Insert("responses", json.RawMessage(`"just a string"`)); err == nil {
		t.Fatal("비객체 본문이 통과됨")
	}
}
