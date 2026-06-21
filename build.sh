#!/usr/bin/env bash
# 교사용 Windows exe 를 빌드한다. 포털/CI(리눅스)에서도 동일하게 동작한다.
# systray 와 SQLite(modernc.org/sqlite) 모두 순수 Go 라 CGO 없이 크로스컴파일된다
# (C 컴파일러 불필요).
set -euo pipefail

OUT="${1:-dist/teaveloper-runner.exe}"
mkdir -p "$(dirname "$OUT")"

# -H windowsgui : 콘솔 창 안 뜸 (더블클릭 UX)
# -s -w         : 디버그 심볼 제거 → exe 크기 축소
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-H windowsgui -s -w" -o "$OUT" .

echo "built: $OUT ($(du -h "$OUT" | cut -f1))"
