// Package appdef 는 AI 가 프론트와 함께 생성하는 ./app/teaveloper.json 을 읽는다.
// 어떤 컬렉션이 있고 각자 어떤 프리셋(권한)인지 선언한다. 러너는 여기 선언된
// 컬렉션만 허용한다(미선언 = 거부 = 어뷰즈 화이트리스트).
package appdef

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Preset 은 컬렉션의 공개 권한 프리셋이다.
type Preset string

const (
	// PresetSubmissions: 외부는 POST(제출)만. 읽기/수정/삭제는 공개 거부.
	// 소유자는 로컬 _admin 에서만 열람. (설문·신청 등)
	PresetSubmissions Preset = "submissions"
	// PresetPublic: 외부가 GET/POST/PATCH/DELETE 전부 가능. (협업 보드 등)
	PresetPublic Preset = "public"
	// PresetPrivate: 외부 접근 전부 거부. 소유자 로컬 _admin 전용.
	PresetPrivate Preset = "private"
)

// publicVerbs 는 프리셋별 외부 방문자에게 허용되는 HTTP 동사다.
// 핵심 안전 가드레일: 프론트가 뭘 호출하든 이 표가 강제한다.
var publicVerbs = map[Preset]map[string]bool{
	PresetSubmissions: {http.MethodPost: true},
	PresetPublic: {
		http.MethodGet: true, http.MethodPost: true,
		http.MethodPatch: true, http.MethodDelete: true,
	},
	PresetPrivate: {},
}

// Def 는 teaveloper.json 의 파싱 결과다.
type Def struct {
	Name        string            `json:"name"`
	Collections map[string]Preset `json:"collections"`
}

// PublicAllows 는 컬렉션에 대해 외부 방문자가 method 를 쓸 수 있는지 본다.
// 미선언 컬렉션이면 (false, false): 두 번째 값은 "선언됨" 여부.
func (d *Def) PublicAllows(collection, method string) (allowed bool, declared bool) {
	preset, ok := d.Collections[collection]
	if !ok {
		return false, false
	}
	return publicVerbs[preset][strings.ToUpper(method)], true
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
	for name, preset := range d.Collections {
		if !nameOK(name) {
			return fmt.Errorf("컬렉션 이름 %q 가 올바르지 않습니다(영문/숫자/_ 만, 1~64자)", name)
		}
		switch preset {
		case PresetSubmissions, PresetPublic, PresetPrivate:
		default:
			return fmt.Errorf("컬렉션 %q 의 프리셋 %q 가 올바르지 않습니다. submissions/public/private 중 하나여야 합니다", name, preset)
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
