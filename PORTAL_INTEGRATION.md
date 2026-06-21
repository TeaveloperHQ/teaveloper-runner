# 포털 연동 인수인계 (teacher-runner → teaveloper-portal)

> **이 문서의 용도**: 러너(`github.com/TeaveloperHQ/teacher-runner`)가 동작하려면
> 포털(`teaveloper.com` / `aramorugeta/teacher-app-portal`)이 무엇을 제공해야 하는지
> 코드 기준으로 정리한 규격이다. 포털 작업 세션은 이 문서를 계약서로 삼아 구현하면
> 된다. (러너 쪽은 구현·검증 완료. 데이터는 교사 PC를 떠나지 않으므로 "결과 전송"이
> 아니라 "연동 규격 전달"이다.)

---

## 0. 한눈에 — 포털이 해야 할 일 체크리스트

- [ ] **A. exe 빌드/배포 파이프라인** — `CGO_ENABLED=0` 윈도우 크로스컴파일(아래 §2)
- [ ] **B. 활성화 시 `config.json` 발급** — 정확한 필드(아래 §1)
- [ ] **C. "내 서버" 활성화 UX** — 다운로드 묶음 + 안내(아래 §4)
- [ ] **D. 게이트웨이 ↔ 포털 endpoint** — `validate`/`offline`(아래 §3, 이미 있으면 확인만)
- [ ] **E. AI 앱 생성 규격 주입** — 프론트가 러너 API를 겨누도록(아래 §5)
- [ ] **F. UI 카피용 보안 설명**(아래 §6)

> 러너가 신규로 요구하는 것은 주로 **A·B·C·E**다. D(validate/offline)는 게이트웨이가
> 이미 호출 중이므로 포털에 이미 있을 가능성이 큼 — 계약만 대조하면 된다.

---

## 1. config.json — 포털이 활성화 때 발급 (★ 정확히 이 형식)

러너는 exe와 **같은 폴더**의 `config.json`을 읽는다. 필드명·타입 고정:

```json
{
  "gatewayUrl": "wss://gw.teaveloper.com/_agent",
  "slug":       "class-abc",
  "publicUrl":  "https://class-abc.teaveloper.com",
  "localPort":  8080,
  "token":      "tnl_xxxxxxxxxxxxxxxxxxxx"
}
```

| 필드 | 타입 | 의미 / 포털이 채우는 값 |
|---|---|---|
| `gatewayUrl` | string | 고정 `wss://gw.teaveloper.com/_agent` (운영). `ws://`/`wss://`만 허용 |
| `slug` | string | 터널 슬러그(영문/숫자/하이픈, **점 불가** — 단일 라벨) |
| `publicUrl` | string | `https://{slug}.teaveloper.com` — 교사·학생에게 보여줄 주소 |
| `localPort` | int(1–65535) | 교사 PC에서 러너가 열 로컬 포트. 기본 `8080` 권장 |
| `token` | string | 베어러 토큰 `tnl_...`. 게이트웨이가 이 토큰으로 검증 |

검증 규칙(러너 측 `internal/config`):
- `gatewayUrl`/`token` 비면 오류 메시지 후 종료. `localPort` 범위 밖이면 종료.
- 파일 없으면 "포털 '내 서버'에서 받은 config.json 을 같은 폴더에 두세요" 안내.

**slug 제약(게이트웨이 `slugFromHost`)**: `{slug}.teaveloper.com`에서 한 단계만 허용.
slug에 `.`이 있으면 라우팅 거부 → slug에 점을 넣지 말 것.

**localPort 주의**: 교사 PC에서 그 포트가 사용 중이면 러너가 "포트 사용 중" 안내 후
종료한다. 포털에서 교사가 포트를 바꿀 수 있게 하거나, 흔치 않은 기본값(예 8080)을 쓸 것.

> (선택) 포털이 교사별 exe에 설정을 구워넣고 싶으면 빌드 시
> `-ldflags "-X .../internal/config.bakedJSON=<json>"` 로 주입 가능. 단 **기본 경로는
> config.json 파일**이며, 파일이 있으면 그게 우선한다.

---

## 2. exe 빌드/배포 — ★ CGO 없이 리눅스 빌더에서

러너는 `getlantern/systray`(트레이)와 `modernc.org/sqlite`(저장소)를 쓰는데 **둘 다
순수 Go**라 C 컴파일러 없이 윈도우 크로스컴파일된다. 포털 빌더(리눅스)에서 그대로:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-H windowsgui -s -w" -o teaveloper-runner.exe .
```
- `-H windowsgui`: 콘솔 창 안 뜸(더블클릭 UX). **반드시 포함**.
- `-s -w`: 심볼 제거 → exe ~11MB.
- 레포에 `build.sh`, `.github/workflows/build.yml`(태그 푸시 시 릴리스 첨부)이 이미 있음.

**배포 형태(권장)**: 교사에게 **단일 exe 1개**를 주고, `config.json`은 활성화 때 별도
다운로드. (또는 exe+config.json+빈 `app/`를 zip으로 묶어 제공 — 폴더 구조를 교사가
헷갈리지 않게 하려면 zip이 더 친절. §4 참조.)

**코드 서명 안 함이 방침** → 포털 안내에 SmartScreen "추가 정보 → 실행" 포함(§6).

---

## 3. 게이트웨이 ↔ 포털 endpoint (게이트웨이가 호출 — 이미 있으면 대조만)

게이트웨이는 에이전트(러너) 접속 시 포털에 토큰을 검증하고, 끊기면 통지한다.
헤더 `x-gateway-secret: {GATEWAY_SHARED_SECRET}`로 인증.

### POST `/api/tunnels/validate`
요청: `{ "token": "tnl_..." }`
응답:
```json
{ "valid": true, "tunnelId": "t1", "slug": "class-abc", "userId": "u1" }
```
- `valid:false` → 게이트웨이가 에이전트를 **403**으로 거절 → 러너는 재시도 없이
  "설정 다시 받으세요" 안내. (토큰 폐기/만료를 여기서 표현)

### POST `/api/tunnels/offline`
요청: `{ "tunnelId": "t1" }` — 러너 연결이 끊겼을 때 통지. 200만 주면 됨.
(포털 대시보드의 온/오프라인 표시에 사용)

> 이 두 endpoint는 게이트웨이가 이미 운영에서 호출 중이므로, 포털에 이미 구현돼 있을
> 것이다. 위 형식과 다르면 게이트웨이 환경변수(`PORTAL_VALIDATE_URL` 등)와 함께 대조.

---

## 4. "내 서버" 활성화 UX (포털 화면)

교사가 손으로 하는 일은 **3가지뿐**이어야 한다: 가입 → 활성화 → exe 실행.

활성화 버튼을 누르면 포털이:
1. 토큰(`tnl_...`) + slug + publicUrl + localPort 발급 → DB 저장(validate가 참조).
2. `config.json` 생성·다운로드 제공(§1 형식).
3. 다운로드 안내 화면:
   - "① 아래 두(세) 파일을 한 폴더에 두세요" — exe, config.json, (빈) `app/`
   - "② AI가 준 앱 파일을 `app` 폴더에 넣으세요"
   - "③ `teaveloper-runner.exe` 더블클릭"
   - SmartScreen "추가 정보 → 실행" 안내(§6)
   - 공개 주소(`publicUrl`)와 "관리 페이지는 내 컴퓨터에서만(localhost) 열림" 설명.

**폴더 구조(교사가 만드는 최종 모습):**
```
📁 (아무 폴더)\
   ├─ teaveloper-runner.exe
   ├─ config.json
   └─ 📁 app\           ← AI가 준 정적 프론트 + teaveloper.json
       ├─ index.html
       └─ teaveloper.json
```
> 친절도를 높이려면 포털이 **exe + config.json + 비어있는 app/ 를 zip**으로 묶어 주고,
> "여기 app 폴더에 AI 파일을 넣으세요"라고 안내하는 게 가장 헷갈림이 적다.

앱 파일이 아직 없어도 러너는 실행되며 "여기에 앱 파일을 넣으세요" 안내 페이지를 띄운다.

---

## 5. AI 앱 생성 규격 — 프론트가 러너 API를 겨누게 (★ 포털의 앱빌더 AI 프롬프트에 주입)

포털의 "AI로 앱 만들기"가 생성하는 산출물은 **정적 프론트 + `teaveloper.json`**이다.
백엔드 코드는 만들지 않는다(러너가 내장). AI는 아래 계약만 지키면 된다.

### 5.1 teaveloper.json (앱과 함께 생성, `app/` 안에)
```json
{
  "name": "우리반 설문",
  "collections": {
    "responses": "submissions",
    "notice":    "public",
    "settings":  "private"
  }
}
```
- `collections`의 키 = 컬렉션 이름(영문/숫자/_ , 1–64자).
- 값 = 프리셋(아래 3종 중 하나). **여기 선언된 컬렉션만** 러너가 허용(미선언 = 404).

### 5.2 데이터 API (러너 제공, 프론트가 fetch)
```
POST   /api/{컬렉션}        레코드 추가 (본문=JSON 객체, 서버가 id·createdAt·updatedAt 부여)
GET    /api/{컬렉션}        목록 (?sort=필드 ?order=desc ?limit=N ?필드=값[정확일치필터])
GET    /api/{컬렉션}/{id}   하나
PATCH  /api/{컬렉션}/{id}   부분 수정(얕은 병합)
DELETE /api/{컬렉션}/{id}   삭제
```
- 응답 객체에는 항상 `id`(string), `createdAt`/`updatedAt`(밀리초 number)가 포함된다.
- 같은 출처(러너가 서빙)에서 호출 → CORS 신경 안 써도 됨. 상대경로 `/api/...` 사용.

### 5.3 프리셋별 "외부 방문자" 허용 동사 (러너가 강제하는 가드레일)
| 프리셋 | 외부(공개 URL)에서 가능 | 소유자(로컬 _admin) |
|---|---|---|
| **submissions** | `POST`만 | 열람·내보내기·삭제 전부 |
| **public** | `GET POST PATCH DELETE` | 전부 |
| **private** | 없음(전부 거부) | 전부 |

- **설문·신청·제출함** → `submissions` (학생은 내기만, 답안 읽기는 교사만).
- **협업 보드·방명록·공용 목록** → `public`.
- **설정·비밀 메모** → `private`(외부 완전 차단, 교사 로컬 전용).
- AI는 "결과를 화면에 보여주는 기능"을 submissions 컬렉션엔 만들면 안 된다(공개 GET이
  403이라 동작 안 함). 결과 열람은 교사용 `/_admin`의 몫임을 프롬프트에 명시.

### 5.4 제한(어뷰즈 방지) — AI가 알아야 할 한계
- 레코드 본문 ≤ 100KB, 컬렉션당 ≤ 50,000개, 공개 IP 레이트리밋 5 req/s(버스트 20).
- **v1: 파일 업로드 미지원**(JSON 레코드만). 스트리밍/SSE/웹소켓 통과 미지원.

### 5.5 프론트 예시(설문)
```html
<form id="f"> … </form>
<script>
f.onsubmit = async e => {
  e.preventDefault();
  const data = Object.fromEntries(new FormData(f));
  const r = await fetch('/api/responses', {           // submissions 프리셋
    method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify(data)
  });
  // 성공 시 r.json() = {id, createdAt, updatedAt, ...입력값}
};
</script>
```

---

## 6. UI 카피용 — 보안/개인정보 설명 (포털 화면 문구로)

- "데이터는 **선생님 컴퓨터에만** 저장됩니다(SQLite). 외부 서버에 올라가지 않습니다."
- "게이트웨이는 내용을 **저장·기록하지 않는** 순수 중계입니다."
- "**관리 페이지(`/_admin`)는 선생님 컴퓨터에서만** 열립니다. 공개 주소로는 절대
  접근할 수 없습니다."(러너가 공개/로컬을 코드 경로로 구분 — 헤더 위조로 못 뚫음)
- SmartScreen: "처음 실행 시 파란 경고가 보이면 **추가 정보 → 실행**을 누르세요."

---

## 7. 러너 쪽 현황 (포털이 다시 안 만들어도 되는 것)

- 정적 서빙(`./app/`, SPA 폴백) + 데이터 API(프리셋 강제) + `/_admin`(CSV/JSON 내보내기)
  + 터널 클라이언트(재연결·writeMu·ping·403 무재시도) = **한 exe, 구현·e2e 검증 완료**.
- teaveloper.json은 **mtime 변경 시 자동 재로딩** → AI가 앱을 수정해도 러너 재시작 불필요.
- 트레이: 상태(🟢/🔴) · 공개주소 열기/복사 · 관리 페이지 열기 · 자동시작 토글 · 종료.

## 8. 포털이 정해줘야 할 열린 사항

1. **localPort 정책**: 고정 기본값(8080)으로 줄지, 교사가 고르게 할지, 충돌 시 안내.
2. **배포 형태**: exe 단독 + config.json 별도 vs exe+config+app/ zip 묶음(권장: zip).
3. **slug 발급 규칙**: 사용자 입력 vs 자동. 점 불가·소문자·중복 방지.
4. **앱 전달 경로**: AI 산출물을 교사가 어떻게 `app/`에 넣는가(zip 다운로드 / 복붙 / 포털이
   대신 묶어주기). 가장 비기술 친화적인 방법 선택.
5. **재활성화/토큰 회전**: 토큰 폐기 시 validate가 false → 러너가 "설정 다시 받기" 안내.
   포털에서 새 config.json 재발급 흐름.
```
