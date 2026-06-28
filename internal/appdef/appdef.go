// Package appdef 는 AI 가 프론트와 함께 생성하는 ./app/teaveloper.json 을 읽는다.
// 어떤 컬렉션이 있고 각자 외부 방문자에게 어떤 권한을 줄지 선언한다. 러너는 여기
// 선언된 컬렉션만 허용한다(미선언 = 거부 = 어뷰즈 화이트리스트).
//
// 컬렉션 값은 두 형식을 모두 받는다(하위호환):
//
//	"responses": "submissions"                                   // 프리셋 문자열(기존)
//	"responses": {"read": false, "write": true, "edit": false}   // 세부 권한(신규)
//
// (고급) 소유자 스코프 — 학생 개별 코드로 자기 것만 열람/수정:
//
//	"results": {"write": true, "owner": {"field": "code", "read": true, "edit": true}}
package appdef

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Preset 은 세부 권한의 친화 단축이다(하위호환용).
type Preset string

const (
	PresetSubmissions Preset = "submissions" // 외부: 쓰기(POST)만
	PresetPublic      Preset = "public"      // 외부: 읽기+쓰기+편집 전부
	PresetPrivate     Preset = "private"     // 외부: 전부 거부
)

// OwnerScope 는 "소유자 코드로 스코프된" 권한이다. 외부 요청이 ?{Field}=코드 를
// 가져오면 그 코드와 일치하는 레코드에 한해 읽기/수정을 허용한다. 코드가 없으면
// 거부(전체 목록 불가), 다른 코드면 그 레코드만(남의 것 못 봄).
type OwnerScope struct {
	Field string `json:"field"` // 소유자 식별 필드(예: "code")
	Read  bool   `json:"read"`  // 자기 코드 레코드 GET
	Edit  bool   `json:"edit"`  // 자기 코드 레코드 PATCH·DELETE
}

// Allows 는 소유자(코드 보유자)가 method 를 쓸 수 있는지 본다.
func (o *OwnerScope) Allows(method string) bool {
	if o == nil {
		return false
	}
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return o.Read
	case http.MethodPatch, http.MethodPut, http.MethodDelete:
		return o.Edit
	}
	return false
}

// CollectionPerm 은 외부(공개 URL) 방문자에게 허용되는 동작이다.
//
//	read  → GET    (목록·상세 조회)
//	write → POST   (새 기록 추가/제출)
//	edit  → PATCH·DELETE (기존 기록 수정·삭제)
//	owner → 위와 별개로, 코드로 스코프된 자기-레코드 읽기/수정
//
// 소유자(로컬 _admin)는 이 값과 무관하게 항상 전체 권한이다.
type CollectionPerm struct {
	Read  bool
	Write bool
	Edit  bool
	Owner *OwnerScope
}

func presetPerm(p Preset) (CollectionPerm, bool) {
	switch p {
	case PresetSubmissions:
		return CollectionPerm{Write: true}, true
	case PresetPublic:
		return CollectionPerm{Read: true, Write: true, Edit: true}, true
	case PresetPrivate:
		return CollectionPerm{}, true
	}
	return CollectionPerm{}, false
}

// UnmarshalJSON 은 프리셋 문자열 또는 {read,write,edit,owner} 객체를 모두 받는다.
func (p *CollectionPerm) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return fmt.Errorf("빈 권한 값")
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		perm, ok := presetPerm(Preset(s))
		if !ok {
			return fmt.Errorf("프리셋 %q 가 올바르지 않습니다(submissions/public/private 중 하나)", s)
		}
		*p = perm
		return nil
	}
	var o struct {
		Read  bool        `json:"read"`
		Write bool        `json:"write"`
		Edit  bool        `json:"edit"`
		Owner *OwnerScope `json:"owner"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return fmt.Errorf("권한 형식 오류(프리셋 문자열이나 {read,write,edit,owner} 객체여야 함): %w", err)
	}
	*p = CollectionPerm{Read: o.Read, Write: o.Write, Edit: o.Edit, Owner: o.Owner}
	return nil
}

// AllowsPublic 은 외부 방문자가 (스코프 없이) method 를 쓸 수 있는지 본다.
func (p CollectionPerm) AllowsPublic(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return p.Read
	case http.MethodPost:
		return p.Write
	case http.MethodPatch, http.MethodPut, http.MethodDelete:
		return p.Edit
	}
	return false
}

// Label 은 관리 화면 표시용 권한 요약.
func (p CollectionPerm) Label() string {
	var base string
	switch (CollectionPerm{Read: p.Read, Write: p.Write, Edit: p.Edit}) {
	case CollectionPerm{Write: true}:
		base = "제출 받기"
	case CollectionPerm{Read: true, Write: true, Edit: true}:
		base = "공개"
	case CollectionPerm{}:
		base = "차단"
	default:
		parts := make([]string, 0, 3)
		if p.Read {
			parts = append(parts, "읽기")
		}
		if p.Write {
			parts = append(parts, "쓰기")
		}
		if p.Edit {
			parts = append(parts, "편집")
		}
		base = strings.Join(parts, "·")
	}
	if p.Owner != nil {
		ow := make([]string, 0, 2)
		if p.Owner.Read {
			ow = append(ow, "읽기")
		}
		if p.Owner.Edit {
			ow = append(ow, "편집")
		}
		base += " + 본인" + strings.Join(ow, "·")
	}
	return base
}

// Def 는 teaveloper.json 의 파싱 결과다.
type Def struct {
	Name        string                    `json:"name"`
	Collections map[string]CollectionPerm `json:"collections"`
}

// Perm 은 컬렉션의 권한과 선언 여부를 반환한다.
func (d *Def) Perm(collection string) (CollectionPerm, bool) {
	p, ok := d.Collections[collection]
	return p, ok
}

// PublicAllows 는 컬렉션에 대해 외부 방문자가 (스코프 없이) method 를 쓸 수 있는지 본다.
// 미선언 컬렉션이면 (false, false): 두 번째 값은 "선언됨" 여부.
func (d *Def) PublicAllows(collection, method string) (allowed bool, declared bool) {
	perm, ok := d.Collections[collection]
	if !ok {
		return false, false
	}
	return perm.AllowsPublic(method), true
}

// Declared 는 컬렉션이 선언돼 있는지 본다.
func (d *Def) Declared(collection string) bool {
	_, ok := d.Collections[collection]
	return ok
}

// Load 는 path(예: ./app/teaveloper.json)를 읽고 검증한다. 파일이 없으면
// (nil, ErrMissing) 을 반환한다 — 호출자가 "앱 파일을 넣으세요" 안내에 사용.
func Load(path string) (*Def, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, fmt.Errorf("teaveloper.json 읽기 실패: %w", err)
	}
	var d Def
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("teaveloper.json 형식 오류(JSON 확인): %w", err)
	}
	if err := d.validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// ErrMissing 은 teaveloper.json 이 없을 때.
var ErrMissing = fmt.Errorf("teaveloper.json 이 없습니다")

func (d *Def) validate() error {
	if len(d.Collections) == 0 {
		return fmt.Errorf("teaveloper.json 에 collections 가 비어 있습니다. 예: {\"collections\":{\"responses\":\"submissions\"}}")
	}
	for name, perm := range d.Collections {
		if !nameOK(name) {
			return fmt.Errorf("컬렉션 이름 %q 가 올바르지 않습니다(영문/숫자/_ 만, 1~64자)", name)
		}
		if perm.Owner != nil && !nameOK(perm.Owner.Field) {
			return fmt.Errorf("컬렉션 %q 의 owner.field %q 가 올바르지 않습니다(영문/숫자/_ 만, 1~64자)", name, perm.Owner.Field)
		}
	}
	return nil
}

func nameOK(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}
