# K-Drama 5분 자동 제작 시스템

Windows용 Wails v2 + Svelte + Go 데스크톱 앱입니다. 로컬 프로젝트 저장, 단계별 재개/오류 복구, 무료 모의 제작, 로컬 자산 가져오기, 기존 Abocado 작업 기록 복구·다운로드, SRT 검사와 300초 FFmpeg 편집을 제공합니다.

기존 `.agents/skills/k-drama-120-auto-studio/SKILL.md`의 원본성·연속성·한국어 립싱크·예산 승인 정책을 반영했습니다. Windows 앱은 로컬 Codex 설정에서 Abocado MCP 주소와 선택적 헤더 헬퍼를 읽습니다. OAuth가 필요한 서버는 앱의 연결 버튼에서 브라우저 승인 후 Windows 사용자 DPAPI로 토큰을 암호화해 저장하고 만료 시 갱신합니다. 토큰은 설정 파일이나 로그에 기록하지 않습니다.

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

앱의 Abocado 연결 테스트는 DNS·인증·history/status 도구를 확인하며 생성 요청을 보내지 않습니다. 연결 후 **완료된 Abocado 작업에서 기존 결과 복구·다운로드**를 선택하면 이 파일럿에 지정된 기존 작업 ID만 조회하고, history에 저장된 S3 객체 URL까지 CDN 다운로드의 대체 경로로 시도합니다. 완료 결과만 복구하며 생성 API를 호출하지 않습니다. 14초보다 긴 음성은 립싱크 전에 FFmpeg로 분할해야 합니다.

## 비용과 승인

`project/estimate.md`는 마지막 비생성 견적과 현재 조건을 담습니다. 유료 생성 구현은 `abocado_check_cost`의 실제 합계, 잔액과 사용자 승인 한도를 확인한 뒤에만 제출해야 합니다. 현재 기본 모드는 mock이며 자동으로 크레딧을 사용하지 않습니다. 1080p 최종 편집은 로컬 FFmpeg 업스케일이고 추가 크레딧이 들지 않지만, 원본 생성/립싱크 해상도는 모델의 허용값을 따릅니다.


