# schat

`schat`은 SSH 세션을 통해 접속하는 경량 채팅 서버입니다. 누구나 SSH 클라이언트만 있으면 별도의 설치 없이 바로 접속해 실시간으로 대화를 나눌 수 있도록 설계되었습니다.

## 주요 기능
- SSH 프로토콜을 이용한 텍스트 기반 채팅 서버
- 다수의 동시 접속자를 지원하는 브로드캐스트 메시징
- 사용자 식별을 돕는 ANSI 색상 닉네임 및 시스템 알림
- 서버 시작 시 호스트 키 자동 생성(기존 키가 없을 때)
- SIGINT/SIGTERM 신호 처리로 부드러운 종료 지원

## 요구 사항
- Go 1.21 이상
- SSH 클라이언트(예: `ssh`, `PuTTY`, `Termius`)

## 빠른 시작
```bash
git clone https://github.com/ledzpl/schat.git
cd schat
go run ./cmd/schat --addr :2222 --host-key configs/ssh_host_rsa
```

호스트 키 파일이 존재하지 않으면 서버가 자동으로 RSA 키를 생성합니다.

### 클라이언트 접속
서버가 실행 중인 호스트에서 다음과 같이 접속할 수 있습니다.

```bash
ssh -p 2222 <사용자명>@localhost
```

- 비밀번호 없이 접속이 허용되며, 입력한 사용자명이 채팅 닉네임으로 사용됩니다.
- 메시지는 입력 후 Enter를 누르면 전송되며, `Ctrl+D`로 세션을 종료할 수 있습니다.

## 실행 옵션
| 옵션 | 기본값 | 설명 |
|------|--------|------|
| `--addr` | `:2222` | SSH 서버가 바인딩할 TCP 주소 |
| `--host-key` | `configs/ssh_host_rsa` | SSH 호스트 프라이빗 키 경로 (없으면 자동 생성) |

## 빌드 & 테스트
- 빌드: `go build ./cmd/schat`
- 단위 테스트: `go test ./...`
- 데이터 레이스 검출: `go test -race ./...`
- 정적 분석(설치되어 있다면): `golangci-lint run`

## 디렉터리 구조
```
cmd/schat          # 실행 엔트리포인트
internal/chat      # 채팅 룸 및 SSH 세션 처리 로직
pkg/sshserver      # SSH 서버 래퍼 및 호스트 키 유틸리티
configs/           # 호스트 키 및 설정 예시
docs/              # 설계 문서 및 런북(필요 시)
testdata/          # 테스트용 고정 데이터
```

## 기여 가이드
1. 변경 사항에 대해 `go test ./...`와 필요 시 `go test -race ./...`를 실행해 주세요.
2. 코드 스타일은 `gofmt` 및 `goimports`를 따라야 합니다.
3. 커밋 메시지는 [Conventional Commits](https://www.conventionalcommits.org/) 규칙을 사용합니다.

이 프로젝트에 대한 제안이나 이슈는 자유롭게 등록해 주세요. SSH 기반 채팅 경험을 더 나아지게 만들기 위한 모든 피드백을 환영합니다.
