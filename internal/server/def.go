package server

import (
	"log"
	"os"
	"path/filepath"

	"github.com/TeaveloperHQ/teacher-runner/internal/appdef"
)

func (s *Server) defPath() string {
	return filepath.Join(s.appDir, "teaveloper.json")
}

// loadDef 는 teaveloper.json 을 읽어 캐시한다. 없으면 def=nil(안내 페이지로 처리).
func (s *Server) loadDef() {
	path := s.defPath()
	info, statErr := os.Stat(path)

	s.defMu.Lock()
	defer s.defMu.Unlock()
	if statErr != nil {
		if s.def != nil {
			log.Printf("teaveloper.json 사라짐 — 앱 미로드 상태로 전환")
		}
		s.def = nil
		s.defModTime = 0
		return
	}
	if s.def != nil && info.ModTime().UnixNano() == s.defModTime {
		return // 변경 없음
	}
	def, err := appdef.Load(path)
	if err != nil {
		log.Printf("teaveloper.json 로드 실패: %v", err)
		s.def = nil
		s.defModTime = 0
		return
	}
	s.def = def
	s.defModTime = info.ModTime().UnixNano()
	log.Printf("앱 로드됨: %q, 컬렉션 %d개", def.Name, len(def.Collections))
}

// currentDef 는 teaveloper.json 의 mtime 이 바뀌었으면 재로딩 후 현재 def 를 반환한다.
// AI 가 앱을 수정해도 재시작 없이 반영된다.
func (s *Server) currentDef() *appdef.Def {
	path := s.defPath()
	info, err := os.Stat(path)

	s.defMu.RLock()
	cached := s.def
	mod := s.defModTime
	s.defMu.RUnlock()

	changed := (err == nil && info.ModTime().UnixNano() != mod) ||
		(err != nil && cached != nil)
	if changed {
		s.loadDef()
		s.defMu.RLock()
		cached = s.def
		s.defMu.RUnlock()
	}
	return cached
}
