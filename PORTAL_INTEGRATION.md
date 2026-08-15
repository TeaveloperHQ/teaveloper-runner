# 포털 연동 인수인계 (teaveloper-runner → teaveloper-portal)

> **이 문서의 용도**: 러너(`github.com/TeaveloperHQ/teaveloper-runner`)가 동작하려면
> 포털(`teaveloper.com` — 소스 비공개)이 무엇을 제공해야 하는지
> 코드 기준으로 정리한 규격이다. 포털 작업 세션은 이 문서를 계약서로 삼아 구현하면
> 된다. (러너 쪽은 구현·검증 완료. 데이터는 교사 PC를 떠나지 않으므로 "결과 전송"이
> 아니라 "연동 규격 전달"이다.)

---

## 0. 한눈에 — 포털이 해야 할 일 체크리스트

- [ ] **A. exe 빌드/배포 파이프라인** — `CGO_ENABLED=0` 윈도우 크로스컴파일(아래 §2)
- [ ] **B. 활성화 시 `config.json` 발급** — 정확한 필드(아래 §1)
- [ ] **C. 활성화 UX — 두 경로** — ★ CLI(Device Flow, 권장) + 수동 다운로드(아래 §4)
- [ ] **D. 게이트웨이 ↔ 포털 endpoint** — `validate`/`offline`(아래 §3, 이미 있으면 확인만)
- [ ] **E. AI 앱 생성 규격 주입** — 프론트가 러너 API를 겨누도록(아래 §5)
- [ ] **F. UI 카피용 보안 설명**(아래 §6)
- [ ] **G. 러너 배포 매니페스트** — Teavel 자동 설치용 고정 URL(아래 §2)

> 러너가 신규로 요구하는 것은 주로 **A·B·C·E**다. D(validate/offline)는 게이트웨이가
> 이미 호출 중이므로 포털에 이미 있을 가능성이 큼 — 계약만 대조하면 된다.
>
> **C·G 가 이번에 새로 들어온 항목이다.** 교사가 손으로 하는 일을 "가입 → 활성화 →
> exe 실행" 3가지에서 **브라우저 승인 클릭 1번**으로 줄이기 위한 것. 근거는 §4 머리말.

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
종료한다. 포털은 교사 PC에서 어느 포트가 비었는지 알 수 없으므로 **아무 값이나(8080)
넣어도 된다** — CLI 활성화(§4.1)를 쓰면 CLI 가 저장 직전에 실제로 빈 포트로 이 필드를
덮어쓴다. 수동 경로(§4.2)에서만 이 값이 그대로 쓰이며, 그때는 충돌 시 교사가 메모장으로
고쳐야 한다(그래서 CLI 경로가 권장이다).

> (선택·비권장) 포털이 교사별 exe에 설정을 구워넣고 싶으면 빌드 시
> `-ldflags "-X .../internal/config.bakedJSON=<json>"` 로 주입 가능. 단 **기본 경로는
> config.json 파일**이며, 파일이 있으면 그게 우선한다.
>
> **CLI 활성화(§4.1)를 도입하면 이 옵션은 쓰지 말 것.** 교사별로 exe 를 굽는 순간
> 배포 파일이 교사마다 달라져 §2 의 고정 URL 배포(§0-G)가 성립하지 않는다. 빌드 주체는
> 지금처럼 포털 그대로 두되, **모두에게 같은 exe 하나**를 내는 것이 전제다.

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

### 2.1 배포 매니페스트 (★ §0-G — Teavel 자동 설치용)

Teavel(교사 PC 세팅 에이전트)이 러너를 **교사 개입 없이** 내려받아 설치하려면 고정
URL 두 개가 필요하다. 사람이 보는 다운로드 페이지와 별개로, 기계가 읽는 경로다.

#### GET `/api/runner/latest`
```json
{
  "version": "0.1.2",
  "url":     "https://teaveloper.com/download/teaveloper-runner-0.1.2-win-amd64.zip",
  "sha256":  "3f7a…",
  "exePath": "teaveloper-runner.exe",
  "notes":   "선택 — 교사에게 보여줄 한 줄 변경 요약"
}
```

| 필드 | 필수 | 의미 |
|---|---|---|
| `version` | ✅ | 유의적 버전. Teavel 이 설치본과 비교해 갱신 여부·`minVersion` 판단 |
| `url` | ✅ | zip 직링크. **인증 불필요**(모두에게 같은 파일이므로 비밀이 없다) |
| `sha256` | ✅ | zip 무결성. **코드 서명을 안 하므로 이게 유일한 위변조 확인 수단** |
| `exePath` | ✅ | zip 내부에서 exe 의 상대 경로 |

- zip 안에는 **exe 만** 넣는다. `config.json` 은 활성화가, `app/` 은 앱 전달이 담당.
- `url` 은 **버전이 박힌 불변 URL**로 준다(캐시·재현성). `latest` 를 덮어쓰지 말 것.
- HTTPS 필수. 리다이렉트를 쓰면 최종 목적지도 HTTPS 여야 한다.

> 이 매니페스트가 성립하려면 **exe 가 모든 교사에게 동일**해야 한다 → §1 의
> `bakedJSON` 을 쓰지 않는 것이 전제다.

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

## 4. 활성화 — 두 경로

활성화란 **교사에게 터널 토큰을 발급하고 그 `config.json`(§1)이 교사 PC 의 exe 옆에
놓이게 하는 일**이다. 두 경로를 모두 제공한다.

| | §4.1 CLI (Device Flow) ★ 권장 | §4.2 수동 (브라우저 다운로드) |
|---|---|---|
| 교사가 하는 일 | **브라우저 승인 클릭 1번** | 가입 → 활성화 → 파일 내려받기 → 폴더로 옮기기 → exe 실행 |
| `config.json` 배치 | CLI 가 제자리에 씀 | 교사가 손으로 옮김 |
| 포트 충돌 | CLI 가 빈 포트로 자동 회피 | 교사가 메모장으로 수정 |
| 자동 실행 등록 | CLI 가 함께 등록 | 교사가 트레이에서 켬 |
| 전제 | Teavel 이 깔려 있어야 함 | 없음 |

**왜 CLI 경로를 권장으로 두는가.** 러너는 원래 "서버 개념이 있는 교사"를 대상으로
설계했지만, 그 교사들조차 **귀찮으면 안 한다.** 수동 경로에는 교사가 포기할 수 있는
지점이 셋 있다 — ① exe 를 처음 더블클릭하면 `config.json` 이 없어 오류 상자만 뜨고
프로그램이 그냥 종료된다(트레이에 뜨지도 않는다), ② 내려받은 `config.json` 을
다운로드 폴더에서 exe 옆으로 **옮겨야** 한다(러너는 exe 폴더에서만 찾는다),
③ `localPort` 가 사용 중이면 또 오류 상자 후 종료된다. CLI 경로는 이 셋을 전부 없앤다.

수동 경로도 계속 유지한다 — Teavel 없이 포털에서 exe 만 받은 교사의 경로이고,
CLI 경로가 실패했을 때의 폴백이기도 하다.

---

### 4.1 CLI 활성화 — Device Flow (★ 신규 구현 대상)

교사 PC 의 CLI(Teavel)가 브라우저를 열어 **짧은 코드**를 보여주고, 교사가 포털에서
승인하면, CLI 가 폴링으로 `config.json` 을 받아 제자리에 쓴다.
OAuth 2.0 Device Authorization Grant(RFC 8628)와 같은 모양이다.

**왜 loopback 리다이렉트(`127.0.0.1/callback`)가 아닌가**: ① 학교 관리 PC 에서 로컬
포트를 열지 않아도 된다(러너가 이미 포트를 물고 있을 수 있다), ② 기본 브라우저가
망가졌거나 GPO 로 막혀 있어도 **휴대폰으로 승인**하면 된다, ③ 포털 구현이 엔드포인트
2개 + 페이지 1개로 끝난다. 대가는 교사가 8자를 한 번 치는 것뿐이다.

#### ① POST `/api/device/code` — CLI 가 시작

요청:
```json
{ "client": "teavel-cli", "clientVersion": "1.2.0" }
```
응답 (200):
```json
{
  "deviceCode":        "dc_9f3a…",
  "userCode":          "WXYZ-1234",
  "verifyUrl":         "https://teaveloper.com/activate",
  "verifyUrlComplete": "https://teaveloper.com/activate?code=WXYZ-1234",
  "interval":          5,
  "expiresIn":         600
}
```

| 필드 | 의미 |
|---|---|
| `deviceCode` | **CLI 만 보관**. 화면·로그에 절대 노출하지 않는다. 폴링의 신원 |
| `userCode` | 교사가 눈으로 보고 입력. 8자 + 하이픈 1개. **혼동 문자 제외**(`0 O 1 I L`) |
| `verifyUrl` | 교사가 손으로 칠 수도 있는 짧은 주소 |
| `verifyUrlComplete` | 코드가 이미 채워진 주소. CLI 가 브라우저로 여는 것은 이쪽 |
| `interval` | 폴링 최소 간격(초) |
| `expiresIn` | `userCode` 유효 시간(초). **600(10분) 권장** |

#### ② POST `/api/device/token` — CLI 가 폴링

요청:
```json
{ "deviceCode": "dc_9f3a…" }
```

응답은 **형식이 올바르면 언제나 HTTP 200**이고 `status` 로 구분한다.
(RFC 8628 은 대기 상태에 400 을 쓰지만, 여기서는 클라이언트 구현을 단순하게 하려고
200 + `status` 로 둔다. 포털 구현자는 이 차이를 유의할 것.)

> **요청 본문은 항상 `Content-Length` 를 달고 온다** — CLI 가 청크 전송을 쓰지 않는다.
> 포털이 `Transfer-Encoding: chunked` 를 처리할 필요가 없다는 뜻이다.
> (구현 중 실제로 밟은 문제다. 청크로 보내면 Content-Length 만 읽는 서버 구현에서
> 본문이 통째로 빈 것으로 보인다.)

| `status` | 뜻 | CLI 동작 |
|---|---|---|
| `pending` | 아직 승인 안 됨 | `interval` 만큼 기다렸다 재시도 |
| `slow_down` | 폴링이 너무 잦음 | `interval` 을 5초 늘리고 재시도 |
| `denied` | 교사가 [거부]를 누름 | 즉시 중단, "취소되었습니다" |
| `expired` | `expiresIn` 초과 | 즉시 중단, 처음부터 다시 안내 |
| `ok` | 승인됨 | `config` 를 저장하고 종료 |

승인 응답:
```json
{
  "status": "ok",
  "config": {
    "gatewayUrl": "wss://gw.teaveloper.com/_agent",
    "slug":       "class-abc",
    "publicUrl":  "https://class-abc.teaveloper.com",
    "localPort":  8080,
    "token":      "tnl_xxxxxxxxxxxxxxxxxxxx"
  }
}
```

- `config` 는 **§1 과 완전히 같은 형식**이다. 새 스키마를 만들지 말 것 — CLI 는 이
  객체를 (localPort 만 빼고) 그대로 파일로 떨군다.
- `localPort` 는 아무 값이나 넣어도 된다. CLI 가 빈 포트로 덮어쓴다(§1 참조).
- **`ok` 는 한 번만.** 반환 즉시 `deviceCode` 를 폐기해 재사용을 막는다.

#### ③ 승인 페이지 `GET /activate`

1. `?code=` 가 있으면 입력칸을 채워 둔다. 없으면 교사가 친다.
2. 로그인 안 돼 있으면 **가입/로그인 먼저**(승인 후 이 화면으로 되돌아온다).
3. 무엇을 승인하는지 명시한다 — 막연한 [승인]이 아니라:
   > **이 컴퓨터를 선생님 서버로 연결합니다.**
   > 주소: `https://class-abc.teaveloper.com`
   > 요청한 곳: Teavel (교사 PC 세팅 도우미)
   > [승인] [거부]
4. 이미 서버가 있는 교사면 **[기존 서버 재연결]** / **[새로 만들기]** 를 고르게 한다
   (재활성화 = §8-5 해소, 아래 "토큰 회전" 참조).

#### 보안 요구사항 (구현 시 반드시)

- **`userCode` 무차별 대입 방지.** 8자라도 온라인 추측이 가능하다고 보고, 승인 화면의
  코드 조회에 IP 당·계정 당 레이트리밋을 걸고 **연속 5회 실패 시 해당 코드를 폐기**한다.
- **`deviceCode` 는 1회용**이며 절대 화면·로그·에러 메시지에 노출하지 않는다.
- 폴링 간격 미준수는 `slow_down` 으로 응답한다(무제한 폴링 차단).
- 전 구간 HTTPS. `token`(`tnl_...`)이 응답 본문에 실리므로 평문 전송 금지.
- 승인 화면은 CSRF 보호를 적용한다(로그인 세션으로 승인이 일어나므로).
- **토큰 회전**: 재활성화로 새 토큰을 내면 **기존 토큰은 즉시 폐기**한다. 그러면
  §3 의 `validate` 가 `false` → 옛 러너는 403 을 받고 재시도 없이 멈춘다(의도된 동작).

#### CLI 쪽이 하는 일 (계약 상대편 — 포털은 구현하지 않음)

포털 구현자가 "그래서 저쪽은 뭘 하나"를 알 수 있게 적어 둔다:

1. `POST /api/device/code`
2. `verifyUrlComplete` 로 브라우저를 연다. **열기에 실패해도 화면에 코드와 주소를 남긴다**
   (다른 기기에서 승인 가능해야 하므로).
3. `interval` 간격으로 `POST /api/device/token` 폴링. `expiresIn` 초과 시 중단.
4. `ok` 를 받으면 `config.localPort` 를 **실제로 비어 있는 포트**로 덮어쓴다.
5. `%LOCALAPPDATA%\Teaveloper\runner\config.json` 에 저장(사용자 전용 ACL).
6. 로그온 자동 실행 등록 —
   `schtasks /Create /F /SC ONLOGON /RL LIMITED /TN TeaveloperRunner /TR "<exe>"`.
   **작업 이름은 반드시 `TeaveloperRunner`** — 러너 트레이의 자동시작 토글이 같은
   이름을 보므로, 이렇게 해야 교사가 나중에 트레이에서 끄고 켜는 것이 일관되게 동작한다.
7. 러너 실행 후 `GET http://127.0.0.1:{localPort}/_admin/api/status` 를 폴링해
   연결을 확인하고 `publicUrl` 을 교사에게 보여준다.
   (이 엔드포인트는 로컬 전용·무인증이며 이미 구현돼 있다 — §7. 다만 `state` 가
   기계용 코드가 아니라 **한국어 표시 문자열**이라는 함정이 있다. §7 참조.)

---

### 4.2 수동 활성화 — "내 서버" UX (포털 화면)

Teavel 없이 포털에서 exe 만 받은 교사의 경로. 교사가 손으로 하는 일은
**3가지뿐**이어야 한다: 가입 → 활성화 → exe 실행.

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
    "board":     "public",
    "settings":  "private",
    "notice":    { "read": true, "write": false, "edit": false }
  }
}
```
- `collections`의 키 = 컬렉션 이름(영문/숫자/_ , 1–64자).
- 값 = **프리셋 문자열**(아래 3종 중 하나) **또는 `{read, write, edit}` 세부 권한 객체**.
  둘 다 받으며 섞어 써도 된다(하위호환). **여기 선언된 컬렉션만** 러너가 허용(미선언 = 404).

### 5.2 데이터 API (러너 제공, 프론트가 fetch)
```
POST   /api/{컬렉션}        레코드 추가 (본문=JSON 객체, 서버가 id·createdAt·updatedAt 부여)
GET    /api/{컬렉션}        목록 (?sort=필드 ?order=desc ?limit=N ?필드=값[정확일치필터])
GET    /api/{컬렉션}/{id}   하나
PATCH  /api/{컬렉션}/{id}   부분 수정(얕은 병합)
DELETE /api/{컬렉션}/{id}   삭제
```
- 응답 객체에는 항상 `id`(string), `createdAt`/`updatedAt`(밀리초 number)가 포함된다.

**프론트 배치 — 두 방식 모두 지원:**
- **A(같은출처)**: 프론트를 러너 `app/` 에 둠 → 상대경로 `/api/...` 호출. CORS 불필요.
- **C(외부 프론트, 확정 모델)**: 프론트를 GitHub Pages 등 외부에 두고
  `https://{slug}.teaveloper.com/api/...` 를 **교차출처로 fetch**. 러너 `/api/*` 가
  **CORS 허용**(`Access-Control-Allow-Origin: *`, `OPTIONS`→204 프리플라이트)하므로
  브라우저에서 바로 호출된다. (v0.1.1+)
- C 방식이라도 **`teaveloper.json` 은 러너 쪽 `app/` 에 있어야** 프리셋이 강제된다
  (프론트가 외부에 있어도 마찬가지). 프리셋/권한은 CORS 와 무관하게 그대로 강제됨.

### 5.3 "외부 방문자" 허용 동사 (러너가 강제하는 가드레일)
세부 권한 → HTTP 동사: `read`=`GET` / `write`=`POST` / `edit`=`PATCH`·`DELETE`.
프리셋은 이 권한들의 단축이다:

| 프리셋 | = 세부 권한 | 외부(공개 URL)에서 가능 | 소유자(로컬 _admin) |
|---|---|---|---|
| **submissions** | `write` | `POST`만 | 열람·내보내기·삭제 전부 |
| **public** | `read+write+edit` | `GET POST PATCH DELETE` | 전부 |
| **private** | (없음) | 없음(전부 거부) | 전부 |

- **소유자(로컬 `/_admin`)는 이 값과 무관하게 항상 전체 권한.**
- **설문·신청·제출함** → `submissions` (학생은 내기만, 답안 읽기는 교사만).
- **협업 보드·방명록·공용 목록** → `public`.
- **설정·비밀 메모** → `private`(외부 완전 차단, 교사 로컬 전용).
- **읽기 전용 공지** 등 조합이 필요하면 `{read:true, write:false, edit:false}` 처럼 세부
  권한으로 선언한다(프리셋에 없는 조합).
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

**§4.1(CLI 활성화) 때문에 러너를 고칠 필요는 없다.** CLI 가 쓰는 두 접점이 이미 있다:

- **연결 확인** — `GET http://127.0.0.1:{localPort}/_admin/api/status` →
  `{state, message, publicUrl, slug, localPort}`. 로컬 전용이라 무인증이며(터널로 들어온
  요청은 404), 파일 존재가 아니라 **터널이 실제로 붙었는지**를 확인할 수 있다.

  ⚠️ **`state` 는 기계용 코드가 아니라 화면에 그대로 쓰는 한국어 문자열이다** —
  `"연결 중" | "연결됨" | "끊김" | "토큰 무효" | "로컬 앱 미응답"`
  (`internal/tunnel/state.go` 의 `State.String()`). CLI 는 지금 `"연결됨"` 을 문자열로
  비교하는데, 표현을 한 글자만 고쳐도 조용히 깨진다.

  → **러너에 추가할 것(작고 덧붙이기만 하는 변경)**: `state` 는 그대로 두고 안정적인
  `code`(`connecting|connected|disconnected|forbidden|localdown`)를 **같이** 내려준다.
  기존 `/_admin` 화면은 `state` 를 계속 쓰므로 하위호환이 깨지지 않는다.
  CLI 는 이미 `code` 가 있으면 그걸 쓰고 없으면 한국어로 되돌리도록 짜여 있어서,
  러너가 언제 넣든 그날부터 자동으로 튼튼해진다.
- **자동 실행** — `internal/autostart` 가 작업 스케줄러 이름 `TeaveloperRunner` 를 쓰고,
  트레이 토글은 그 작업의 **존재 여부만** 본다. 외부 설치자가 같은 이름으로 등록해 두면
  트레이 체크박스가 켜진 상태로 보이고, 교사가 끄는 것도 그대로 동작한다.
  (`/RL LIMITED` 이므로 관리자 권한 불필요.)

## 8. 포털이 정해줘야 할 열린 사항

1. ~~**localPort 정책**~~ — **해소(§4.1)**. 포털은 아무 값이나 넣고, CLI 가 빈 포트로
   덮어쓴다. 수동 경로(§4.2)에서만 기본값 `8080` 이 그대로 쓰인다.
2. **배포 형태**: 사람이 받는 다운로드는 exe 단독 vs zip 묶음(권장: zip).
   기계가 받는 경로는 §2.1 매니페스트로 확정.
3. **slug 발급 규칙**: 사용자 입력 vs 자동. 점 불가·소문자·중복 방지.
   (CLI 경로에서는 교사가 slug 를 고를 화면이 승인 페이지뿐이므로, **자동 발급 +
   나중에 포털에서 변경**이 CLI 흐름과 가장 잘 맞는다.)
4. **앱 전달 경로**: AI 산출물을 교사가 어떻게 `app/`에 넣는가(zip 다운로드 / 복붙 /
   포털이 대신 묶어주기). 가장 비기술 친화적인 방법 선택.
   → §4.1 이 자리 잡으면 **같은 방식으로 풀 수 있다**: 활성화 토큰이 이미 교사 PC 에
   있으므로 인증이 공짜고, `teavel 러너 앱 받기` 가 zip 을 받아 `app/` 에 풀면 된다.
   활성화(평생 1회)와 앱 전달(수업마다)은 수명이 다르므로 **별도 단계로 둔다.**
5. ~~**재활성화/토큰 회전**~~ — **해소(§4.1)**. 같은 Device Flow 를 다시 돌리고,
   승인 화면에서 [기존 서버 재연결] 을 고르면 새 토큰 발급 + 기존 토큰 즉시 폐기.
   옛 러너는 §3 `validate` 가 `false` 를 주어 403 으로 멈춘다.
