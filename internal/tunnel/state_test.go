package tunnel

import "testing"

// Code 는 /_admin/api/status 로 나가 외부 도구(Teavel)가 연결 여부를 판단하는 계약이다.
// 철자가 바뀌면 그 도구들이 조용히 깨지므로 값을 여기에 못 박아 둔다.
func TestStateCodeIsStable(t *testing.T) {
	want := map[State]string{
		StateConnecting:   "connecting",
		StateConnected:    "connected",
		StateDisconnected: "disconnected",
		StateForbidden:    "forbidden",
		StateLocalDown:    "localdown",
	}
	for state, code := range want {
		if got := state.Code(); got != code {
			t.Errorf("State(%d).Code() = %q, 원하는 값 %q", state, got, code)
		}
	}
	if got := State(99).Code(); got != "unknown" {
		t.Errorf("모르는 상태의 Code() = %q, 원하는 값 %q", got, "unknown")
	}
}

// 화면용 문자열과 기계용 코드가 섞이지 않았는지 — 한쪽을 고치다 다른 쪽을 건드리면 잡힌다.
func TestStateStringIsNotCode(t *testing.T) {
	for _, s := range []State{StateConnecting, StateConnected, StateDisconnected, StateForbidden, StateLocalDown} {
		if s.String() == s.Code() {
			t.Errorf("State(%d): String() 과 Code() 가 같다(%q) — 화면용/기계용이 섞였다", s, s.Code())
		}
	}
}
