# K-Drama 5분 자동 제작 시스템

Windows용 Wails v2 + Svelte + Go 데스크톱 앱입니다. 로컬 프로젝트 저장, 단계별 재개/오류 복구, 무료 모의 제작, 로컬 자산 가져오기, 기존 Abocado 작업 기록 복구·다운로드, SRT 검사와 300초 FFmpeg 편집을 제공합니다.

기존 `.agents/skills/k-drama-120-auto-studio/SKILL.md`의 원본성·연속성·한국어 립싱크·예산 승인 정책을 반영했습니다. 로컬 Codex 설정의 Abocado URL을 읽어 Streamable HTTP MCP를 연결하고, 설정에 토큰 환경변수가 있으면 메모리에서만 사용합니다. 앱은 비밀값을 파일에 저장하거나 출력하지 않습니다. 조직의 Codex 로그인이 MCP 토큰으로 공개되지 않는 환경에서는 서버가 직접 연결을 거부할 수 있습니다.

## 준비

- Windows 10/11, Go 1.26 이상, Node.js, WebView2 Runtime
- Wails v2 CLI
- 실제 Abocado 생성을 쓰려면 연결 가능한 네트워크와 MCP 서버 인증 수단
- 실제 MP4 렌더에는 FFmpeg 실행 파일

## 실행 및 검증

```powershell
cd frontend
npm install
npm run check
npm run build
cd ..
go test ./...
cd backend
go test ./...
cd ..
wails build
```

Wails 바이너리는 `build/bin/kdrama120.exe`에 생성됩니다. 독립형 무료 모의 실행은 다음과 같습니다.

```powershell
cd backend
go run ./cmd/agent -workspace ./data
```

기존 미디어 자산은 앱에서 로컬 폴더를 선택하거나, Codex 설정에 연결할 수 있을 때 **Abocado 작업 복구·다운로드**를 누르면 됩니다. 복구는 완료된 기록만 조회하고 생성 API를 호출하지 않습니다. 14초보다 긴 음성은 립싱크 전에 FFmpeg로 분할해야 합니다.

## 비용과 승인

`project/estimate.md`는 마지막 비생성 견적과 현재 조건을 담습니다. 유료 생성 구현은 `abocado_check_cost`의 실제 합계, 잔액과 사용자 승인 한도를 확인한 뒤에만 제출해야 합니다. 현재 기본 모드는 mock이며 자동으로 크레딧을 사용하지 않습니다. 1080p 최종 편집은 로컬 FFmpeg 업스케일이고 추가 크레딧이 들지 않지만, 원본 생성/립싱크 해상도는 모델의 허용값을 따릅니다.


