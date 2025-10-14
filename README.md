# schat

`schat`은 SSH 세션으로 접속하는 실시간 채팅 서버입니다. SSH 클라이언트만 있으면 추가 설치 없이 채팅에 참여할 수 있도록 최소 의존성으로 설계되었습니다.

## 주요 기능
- SSH 프로토콜 기반 단일 바이너리: 별도 포트 포워딩이나 브라우저 없이 `ssh` 명령만으로 접속할 수 있습니다.
- 실시간 브로드캐스트: 다수의 동시 접속자에게 타임스탬프와 ANSI 색상이 포함된 메시지를 전달합니다.
- 터미널 친화 UI: 접속자 수 상태 표시, 입력 줄 버퍼, 백스페이스 및 `Ctrl+C`/`Ctrl+D` 같은 제어 키를 지원합니다.
- 자동 호스트 키 관리: 지정 경로에 RSA 호스트 키가 없으면 안전한 권한으로 새 키를 생성합니다.
- 우아한 종료: `SIGINT`/`SIGTERM`을 처리해 세션을 정리한 뒤 안전하게 종료합니다.

## 필요 조건
- Go 1.21 이상
- SSH 클라이언트(`ssh`, `PuTTY`, `Termius` 등)
- (선택) 정적 분석을 위한 `golangci-lint`

## 빠르게 시작하기
```bash
git clone https://github.com/ledzpl/schat.git
cd schat
go run ./cmd/schat --addr :2222 --host-key configs/ssh_host_rsa
```

- `--addr`: SSH 서버가 바인딩할 TCP 주소 (기본값 `:2222`)
- `--host-key`: SSH 호스트 프라이빗 키 경로. 파일이 없으면 2048비트 RSA 키를 생성하고 `0600` 권한으로 저장합니다. 개발 중 임시 키를 쓰고 싶다면 빈 문자열을 넘겨 `--host-key ""`처럼 실행하세요.

### 실행 바이너리 빌드
```bash
go build -o bin/schat ./cmd/schat
```
생성된 바이너리를 통해 동일한 옵션으로 서버를 실행할 수 있습니다.

### SSH 클라이언트에서 접속
```bash
ssh -p 2222 <닉네임>@localhost
```
- SSH 사용자명은 채팅 닉네임으로 사용됩니다.
- 메시지를 입력하고 Enter를 누르면 전송되며, `Ctrl+D`로 세션을 종료할 수 있습니다. `Ctrl+C`는 현재 입력 줄을 비우고 안내 메시지를 출력합니다.

## 프로젝트 구조
```
cmd/schat/main.go    # 실행 엔트리포인트 및 서버 부팅 로직
internal/chat/       # 세션, 채팅방, 터미널 UI 등 대화 도메인 로직
pkg/sshserver/       # SSH 리스너, 호스트 키 로딩/생성 유틸리티
configs/ssh_host_rsa # 개발용 호스트 키 예시(운영 환경에서는 새 키를 생성하세요)
```

## 개발 가이드
- 의존성 정리: `go mod tidy`
- 빌드 확인: `go build ./cmd/schat`
- 단위 테스트: `go test ./...`
- 데이터 레이스 검출: `go test -race ./...`
- 정적 분석: `golangci-lint run`
- 커밋 전에는 `gofmt`와 `goimports`를 적용하고, 메시지는 Conventional Commits 형식을 사용합니다.

## 테스트
핵심 채팅 동작에 대한 단위 테스트는 `internal/chat` 패키지에 위치합니다. 새 동작을 추가할 때는 테이블 기반 테스트와 필요한 픽스처를 `testdata/`에 추가해 주세요.

## 라이선스
이 프로젝트는 MIT 라이선스 하에 배포됩니다. 자세한 내용은 `LICENSE` 파일을 참고하세요.

---

# schat (English)

`schat` is a real-time chat server reachable over SSH sessions. With only an SSH client, contributors can join a shared conversation without installing extra tools.

## Key Features
- Single binary over SSH: join the chat with the `ssh` command—no browser or additional forwarding required.
- Real-time broadcasting: distributes timestamped messages with ANSI color tags to every connected participant.
- Terminal-friendly UI: shows online user counts, maintains an input buffer, and respects backspace plus controls like `Ctrl+C`/`Ctrl+D`.
- Automatic host-key management: generates a new RSA host key at the configured path when missing, storing it with safe permissions.
- Graceful shutdown: traps `SIGINT`/`SIGTERM`, cleans up sessions, and stops the server without dropping state abruptly.

## Requirements
- Go 1.21 or later
- SSH client (`ssh`, `PuTTY`, `Termius`, etc.)
- (Optional) `golangci-lint` for static analysis

## Quick Start
```bash
git clone https://github.com/ledzpl/schat.git
cd schat
go run ./cmd/schat --addr :2222 --host-key configs/ssh_host_rsa
```

- `--addr`: TCP address the SSH server binds to (default `:2222`)
- `--host-key`: path to the SSH host private key. When the file is absent, a 2048-bit RSA key is generated and stored with `0600` permissions. Pass an empty string like `--host-key ""` to use an ephemeral key during development.

### Build the Binary
```bash
go build -o bin/schat ./cmd/schat
```
Launch the produced binary with the same flags to run the server.

### Connect from an SSH Client
```bash
ssh -p 2222 <nickname>@localhost
```
- The SSH username becomes the chat nickname.
- Type a message and press Enter to send. Use `Ctrl+D` to exit; `Ctrl+C` clears the current input line and prints a hint.

## Project Layout
```
cmd/schat/main.go    # Application entrypoint and server bootstrap logic
internal/chat/       # Session flow, chat room management, terminal UI
pkg/sshserver/       # SSH listener wrapper plus host-key utilities
configs/ssh_host_rsa # Example host key (generate a new one for production)
```

## Developer Guide
- Manage dependencies: `go mod tidy`
- Verify builds: `go build ./cmd/schat`
- Run unit tests: `go test ./...`
- Detect data races: `go test -race ./...`
- Perform static checks: `golangci-lint run`
- Format with `gofmt`/`goimports` and follow Conventional Commits for messages.

## Testing
Unit tests for core chat behavior live in `internal/chat`. When adding features, prefer table-driven cases and store any required fixtures under `testdata/`.

## License
Distributed under the MIT License. Refer to the `LICENSE` file for the full text.
