package appdef

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 프리셋 문자열(기존)과 세부 권한 객체(신규)가 모두 같은 결과로 파싱되는지.
func TestParsePresetAndGranular(t *testing.T) {
	raw := `{"name":"t","collections":{
		"resp":"submissions",
		"board":"public",
		"secret":"private",
		"notice":{"read":true,"write":false,"edit":false},
		"full":{"read":true,"write":true,"edit":true}
	}}`
	var d Def
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := d.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	want := map[string]CollectionPerm{
		"resp":   {Write: true},                          // submissions == write-only
		"board":  {Read: true, Write: true, Edit: true},  // public == all
		"secret": {},                                      // private == none
		"notice": {Read: true},                            // granular read-only
		"full":   {Read: true, Write: true, Edit: true},
	}
	for name, w := range want {
		if got := d.Collections[name]; got != w {
			t.Errorf("%s: got %+v want %+v", name, got, w)
		}
	}
}

// verb 집행: read=GET, write=POST, edit=PATCH/DELETE.
func TestPublicAllows(t *testing.T) {
	d := &Def{Collections: map[string]CollectionPerm{
		"sub":    {Write: true},
		"notice": {Read: true},
		"pub":    {Read: true, Write: true, Edit: true},
		"none":   {},
	}}
	cases := []struct {
		col, method string
		want        bool
	}{
		{"sub", http.MethodPost, true},
		{"sub", http.MethodGet, false},   // 제출만 — 읽기 금지
		{"sub", http.MethodPatch, false}, // 편집 금지
		{"notice", http.MethodGet, true},
		{"notice", http.MethodPost, false}, // 읽기 전용 — 쓰기 금지
		{"pub", http.MethodDelete, true},
		{"none", http.MethodGet, false},
		{"none", http.MethodPost, false},
	}
	for _, c := range cases {
		got, declared := d.PublicAllows(c.col, c.method)
		if !declared {
			t.Errorf("%s 미선언으로 나옴", c.col)
		}
		if got != c.want {
			t.Errorf("PublicAllows(%s,%s)=%v want %v", c.col, c.method, got, c.want)
		}
	}
	// 미선언 컬렉션
	if _, declared := d.PublicAllows("ghost", http.MethodGet); declared {
		t.Error("미선언 컬렉션이 declared=true")
	}
}

// 소유자 스코프(개별 코드 열람) 파싱·권한.
func TestOwnerScope(t *testing.T) {
	raw := `{"collections":{"results":{"write":true,"owner":{"field":"code","read":true,"edit":true}}}}`
	var d Def
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	if err := d.validate(); err != nil {
		t.Fatal(err)
	}
	p := d.Collections["results"]
	if p.Owner == nil || p.Owner.Field != "code" || !p.Owner.Read || !p.Owner.Edit {
		t.Fatalf("owner 파싱 실패: %+v", p.Owner)
	}
	// 비스코프(누구나): write 만, read 는 막힘
	if !p.AllowsPublic("POST") {
		t.Error("write 는 누구나 허용돼야")
	}
	if p.AllowsPublic("GET") {
		t.Error("비스코프 read 는 막혀야")
	}
	// 소유자(코드 보유자): read+edit, write 아님
	if !p.Owner.Allows("GET") || !p.Owner.Allows("PATCH") || !p.Owner.Allows("DELETE") {
		t.Error("owner read/edit 허용돼야")
	}
	if p.Owner.Allows("POST") {
		t.Error("owner 는 POST(write) 가 아님")
	}
	// 빈 owner.field 는 거부
	var d2 Def
	if json.Unmarshal([]byte(`{"collections":{"x":{"owner":{"field":"","read":true}}}}`), &d2) == nil {
		if d2.validate() == nil {
			t.Error("빈 owner.field 가 통과됨")
		}
	}
	// owner 라벨
	op := CollectionPerm{Write: true, Owner: &OwnerScope{Field: "code", Read: true, Edit: true}}
	if got := op.Label(); got != "제출 받기 + 본인읽기·편집" {
		t.Errorf("owner label=%q", got)
	}
}

// 잘못된 프리셋 문자열은 거부.
func TestBadPreset(t *testing.T) {
	var d Def
	err := json.Unmarshal([]byte(`{"collections":{"x":"open"}}`), &d)
	if err == nil {
		t.Fatal("잘못된 프리셋이 통과됨")
	}
}

// Label: 프리셋 일치 시 친화명, 아니면 조합.
func TestLabel(t *testing.T) {
	cases := map[CollectionPerm]string{
		{Write: true}:                         "제출 받기",
		{Read: true, Write: true, Edit: true}: "공개",
		{}:                                     "차단",
		{Read: true}:                           "읽기",
		{Read: true, Edit: true}:               "읽기·편집",
	}
	for p, want := range cases {
		if got := p.Label(); got != want {
			t.Errorf("Label(%+v)=%q want %q", p, got, want)
		}
	}
}
