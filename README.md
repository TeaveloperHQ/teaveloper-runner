<p align="center">
  <img src="assets/logo.png" alt="Teaveloper Runner" width="520">
</p>

<p align="center">
  <a href="https://github.com/TeaveloperHQ/teacher-runner/actions/workflows/build.yml"><img src="https://github.com/TeaveloperHQ/teacher-runner/actions/workflows/build.yml/badge.svg" alt="ci"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="license: MIT"></a>
  <img src="https://img.shields.io/badge/platform-Windows-0078D6" alt="platform">
</p>

<p align="center"><sub>배포용 exe 는 <b>teaveloper 포털에서 빌드</b>합니다(앱 이름 통일성). 이 레포는 소스이며 GitHub 릴리스로 exe 를 배포하지 않습니다.</sub></p>

# Teaveloper 러너 (teacher-runner)

내 컴퓨터를 **공짜 서버**로 만들어, AI가 만들어 준 앱(설문·기록·신청 등)을 외부
인터넷에서 접속할 수 있게 해주는 작은 프로그램입니다.

- **데이터는 내 컴퓨터에만 저장**됩니다(SQLite). 외부 서버에 올리지 않습니다.
- 앱의 **백엔드(데이터 저장)는 이 러너가 내장 제공**합니다. 임의 코드를 실행하지
  않으므로 안전합니다("터미널 0").
- 한 개의 exe 안에 **정적 서빙 + 데이터 API + 관리 페이지 + 터널**이 모두 들어 있습니다.

> 설치 필요 없음. **exe 하나 더블클릭** → 끝.

---

## 👩‍🏫 선생님용 사용법 (손으로 하는 일은 3가지뿐)

### ① 활성화 — 설정 파일 받기
포털([teaveloper.com](https://teaveloper.com)) → 가입 → **내 서버** → 서비스 활성화 →
`config.json` 다운로드.

### ② 앱 파일 넣기
AI가 만들어 준 앱 파일을 `app` 폴더에 넣습니다. 폴더 구조는 이렇게:

```
📁 내폴더\
   ├─ teaveloper-runner.exe   ← 프로그램
   ├─ config.json             ← 포털에서 받은 설정
   └─ 📁 app\                  ← AI가 준 앱 파일을 여기에
       ├─ index.html
       └─ teaveloper.json     ← AI가 함께 만들어 줌(데이터 선언)
```

*(스크린샷 자리: 폴더 구조)*

> 아직 앱 파일이 없어도 괜찮습니다. 그냥 실행하면 "여기에 앱 파일을 넣으세요"
> 안내 페이지가 나옵니다.

### ③ 실행 — teaveloper-runner.exe 더블클릭
오른쪽 아래 **시스템 트레이**에 동그란 아이콘이 생깁니다.

| 아이콘 | 뜻 |
|:---:|---|
| 🟢 초록 | 연결됨 — 외부에서 접속 가능 |
| 🟡 노랑 | 연결 중 |
| 🔴 빨강 | 끊김(자동 재연결 중) 또는 설정 만료 |

아이콘을 **오른쪽 클릭**하면:
- **🌐 공개 주소 열기 / 📋 복사** — 학생에게 알려줄 주소
- **🗂 관리 페이지 열기** — 결과 열람·내보내기(내 컴퓨터에서만)
- **Windows 시작 시 자동 실행** — 켜기/끄기
- **종료**

이제 `config.json` 의 `publicUrl`(예: `https://class-abc.teaveloper.com`)을 학생에게
알려주면 됩니다.

### 결과 보기 / 내보내기
트레이 → **관리 페이지 열기**(또는 `http://localhost:<포트>/_admin`).
컬렉션별 결과 표·검색, **CSV/JSON 내보내기**, 삭제가 가능합니다.
이 페이지는 **내 컴퓨터에서만** 열립니다(외부에 절대 노출되지 않음).

---

## ⚠️ "Windows의 PC 보호" 경고가 떠요
코드 서명을 하지 않아 처음 실행 시 파란 경고가 뜰 수 있습니다(정상).
**추가 정보 → 실행** 을 누르세요. 백신이 막으면 예외(허용)로 등록하세요.

*(스크린샷 자리: SmartScreen "추가 정보 → 실행")*

---

## 🤖 데이터 API 계약 (AI가 앱을 만들 때 겨누는 규격)

### teaveloper.json — 데이터 선언 (앱과 함께 생성)
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
러너는 **선언된 컬렉션만** 허용합니다(미선언 컬렉션 요청은 404).

### API 표면 (러너 제공, 터널로 공개)
```
POST   /api/{컬렉션}        레코드 추가 (서버가 id·createdAt 자동 부여, 본문=JSON 객체)
GET    /api/{컬렉션}        목록 (?sort= ?order=desc ?limit= ?필드=값 필터)
GET    /api/{컬렉션}/{id}   하나
PATCH  /api/{컬렉션}/{id}   부분 수정(얕은 병합)
DELETE /api/{컬렉션}/{id}   삭제
```
모든 레코드에는 서버가 `id`, `createdAt`, `updatedAt`(밀리초)를 넣어 돌려줍니다.

### 프리셋별 "외부 방문자" 허용 동사 (러너가 강제하는 가드레일)
| 프리셋 | 외부가 할 수 있는 것 | 소유자(로컬 _admin) |
|---|---|---|
| **submissions** | `POST`(제출)만 | 전부(열람·내보내기·삭제) |
| **public** | `GET POST PATCH DELETE` | 전부 |
| **private** | 없음(전부 거부) | 전부 |

- 프론트가 무엇을 호출하든 러너가 동사를 막습니다.
- **submissions 의 결과 읽기는 공개 API에 없습니다.** 오직 로컬 `/_admin` 에서만 →
  학생 답안이 공개 URL로 새지 않습니다.

### 제한 (어뷰즈 방지)
- 레코드 본문 ≤ 100KB, 컬렉션당 ≤ 50,000개, 공개 IP 레이트리밋(5 req/s, 버스트 20).
- v1: **파일 업로드 미지원**(JSON 레코드만). 스트리밍/SSE 미지원.

---

## 🔒 보안 / 개인정보 (어떻게 "데이터가 내 PC에만" 인가)
- 데이터는 내 컴퓨터의 `teaveloper-data.db`(SQLite)에만 저장됩니다.
- 게이트웨이는 요청/응답 **본문을 저장·기록하지 않는** 순수 중계입니다.
- **공개 요청과 로컬 요청은 코드 경로로 구분**됩니다(헤더 추론 아님). 터널로 들어온
  요청에는 "공개" 표식이 박혀 `/_admin` 에 **물리적으로 도달할 수 없고**, 프리셋
  가드레일이 강제됩니다. 관리 리스너는 `127.0.0.1` 에만 바인딩됩니다.

---

## 🛠 개발자용

### 구조
```
main.go                  config 로드 → store 열기 → server 생성 → loopback 리스너 → UI
internal/config/         config.json 로드·검증, baseDir(app/·db 기준)
internal/store/          SQLite(modernc, 무CGO) records CRUD
internal/appdef/         teaveloper.json 파서 + 프리셋 권한 맵
internal/server/         mux: 정적 + /api(프리셋 게이트) + /_admin, 공개/로컬 context
  dispatch.go              터널 req → in-process ServeHTTP (공개 표식 주입)
  admin.go + admin.html    관리 페이지 + CSV/JSON 내보내기
  rate.go                  IP 레이트리밋
internal/tunnel/         WS 전송 전용(재연결·writeMu·ping·403), 핸들러 주입
internal/ui/             windows=systray 트레이 / 그 외=콘솔(개발 테스트)
internal/autostart/      windows=schtasks 로그온 트리거
app/                     예시 설문 프론트 + teaveloper.json
```

### 빌드 (Windows exe — 포털/CI 리눅스에서 동일, CGO 불필요)
```bash
./build.sh                 # → dist/teaveloper-runner.exe
# 또는 수동:
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-H windowsgui -s -w" -o dist/teaveloper-runner.exe .
```
> systray 의 Windows 백엔드와 `modernc.org/sqlite` 모두 **순수 Go** → C 컴파일러
> 없이 빌드됩니다. 포털 빌드 파이프라인(리눅스)에서 그대로 떨어집니다. exe ~11MB.

### 로컬 테스트 (리눅스/macOS, 콘솔 모드)
참조 게이트웨이(`github.com/aramorugeta/teacher-app-portal` 의 `gateway/`)를 띄우고,
`config.json`+`app/` 이 있는 폴더에서 러너를 실행한 뒤 `Host: <slug>` 로 왕복 확인.

검증 완료: 정적 서빙 / submissions(공개 POST 201, 공개 GET·DELETE 403) /
public 프리셋 전체 CRUD / private 전체 거부 / 미선언 컬렉션 404 /
`/_admin` 터널 차단(404)·로컬 정상 / CSV 내보내기 / 50 동시요청(레이트리밋·무패닉) /
재연결·403 무재시도.

### 프로토콜
게이트웨이 ↔ 러너는 단일 WebSocket 위 JSON 프레임(req/res/err)으로 요청을 `id`
다중화(본문 base64). 인증 `Authorization: Bearer {token}`(403 시 재시도 안 함),
쓰기 직렬화(writeMu) 필수, 30s ping/60s 무응답 끊김. 상세는 참조 레포
`gateway/README.md`.

### 아이콘
앱/트레이 아이콘은 `assets/`에 있다(`icon.ico` 멀티사이즈, `icon.png`, 배너 `logo.png`).
exe 아이콘은 `rsrc_windows_amd64.syso`로 임베드되며, Go 툴체인이 윈도우/amd64 빌드 시
자동 링크한다(CI 추가 설정 불필요). 아이콘을 바꾸려면:
```bash
rsrc -ico assets/icon.ico -arch amd64 -o rsrc_windows_amd64.syso
```

### 연관 프로젝트
- **게이트웨이/포털**: `github.com/aramorugeta/teacher-app-portal` (무저장 중계 + 토큰 발급).
- **포털 연동 규격**: [PORTAL_INTEGRATION.md](PORTAL_INTEGRATION.md) — 포털이 제공해야 할
  것(config.json·빌드·활성화 UX·AI 앱 생성 규격)을 계약으로 정리.

---

## 📄 라이선스

[MIT](LICENSE) © 2026 TeaveloperHQ
