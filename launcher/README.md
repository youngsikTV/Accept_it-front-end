# ACCEPT IT 실행기 (launcher)

조달청 홍보용 토론 게임 **ACCEPT IT**을 **`AcceptIt.exe` 하나로 실행**하고, 게임에 나오는 **물품을 직접 적은 목록(`items.txt`)에서 무작위로 출제**하게 해 주는 실행기입니다.

> 이 폴더(`launcher/`)는 이 README와 함께 새로 추가된 것입니다. 루트의 `README.md`는 수정하지 않았습니다.

## 이번 변경 사항 (수정사항)

| 구분 | 내용 |
|---|---|
| 백엔드(`src/`) | **변경 없음** |
| 프론트엔드(`frontend/`) | **변경 없음** |
| `launcher/` 추가 | Go로 만든 실행기(`AcceptIt.exe`)와 소스(`_source/`), 물품 목록(`items.txt`), 배포 폴더 생성 스크립트(`make-bundle.bat/.ps1`), 사용법(`사용법.txt`) |
| `.gitignore` | 실행기가 만드는 생성물/실행 데이터(`/dist/`, `/runtime/`, `/data/` 등) 제외 추가 |

### 실행기가 하는 일
1. `items.txt`의 물품을 DB의 `procurement_item` 표에 반영합니다. 목록에 없는 물품은, 지난 게임 기록이 참조하지 않는 것만 삭제합니다. 백엔드는 게임 시작 시 이 표에서 무작위로 뽑으므로(`RoomService.startGame`) **백엔드 코드를 바꾸지 않고** 출제 물품이 바뀝니다.
2. MySQL과 백엔드를 켭니다.
3. 게임 화면(`VITE_USE_MOCK=false`로 빌드한 프론트)을 내장해서 제공하고, `/api`와 `/ws`는 백엔드로 연결합니다.
4. 3307 / 8080 / 5173 포트가 이미 쓰이고 있으면 빈 포트를 자동으로 골라 씁니다. (설치 없는 모드)
5. 한글이 들어간 폴더 경로면 `Z:` 같은 가상 드라이브로 연결해 실행합니다. (설치 없는 모드, Windows)

## 폴더 구성

```
launcher/
  AcceptIt.exe        실행기 (Windows x64)
  items.txt           출제할 물품 목록  ← 직접 수정
  keywords.txt        키워드 카드 목록(찬성/반대 40개)  ← 직접 수정
  make-bundle.bat     배포용 폴더/zip 만들기 (make-bundle.ps1 실행)
  make-bundle.ps1
  사용법.txt
  _source/            실행기 소스 (Go) + 내장 프론트 빌드(web/)
```

## 실행 방법

실행기는 **두 가지 모드**를 자동으로 고릅니다.

| 모드 | 조건 | 필요한 것 |
|---|---|---|
| 설치 없이 실행 | `AcceptIt.exe` 옆에 `runtime/app.jar`가 있음 | 없음 (파일만 있으면 됨) |
| Docker 방식 (개발용) | `runtime` 폴더 없이 프로젝트 폴더 안에서 실행 | Docker Desktop, Java 21 |

### A. 개발용 — Docker 방식
1. Docker Desktop을 켭니다.
2. 이 `launcher` 폴더가 프로젝트 폴더(`build.gradle`, `docker-compose.yml`이 있는 곳) 안에 있는 상태에서 `launcher\AcceptIt.exe`를 더블클릭합니다.
3. 처음에는 몇 분 걸립니다. 끝나면 브라우저에 게임이 열립니다.

자동으로 처리하는 것: `.env`가 없으면 생성(있으면 덮어쓰지 않음), 3307/8080을 쓰는 다른 컨테이너는 물은 뒤 잠시 정지(삭제하지 않음), DB 계정이 `.env`와 다르면 초기화 여부를 물음.

### B. 다른 사람에게 줄 설치 없는 실행 폴더 만들기 (개발자 PC에서 1번)
1. `launcher` 폴더를 프로젝트 폴더 안에 둡니다. (JDK 21, 인터넷 필요)
2. `launcher\make-bundle.bat`을 더블클릭합니다.
   - `gradlew bootJar -x test`로 백엔드 jar 생성
   - Java 21(JRE), MySQL 8.0(ZIP)을 내려받아 `runtime`에 배치
   - `dist\AcceptIt.zip`(약 300~400MB) 생성
3. **`dist\AcceptIt.zip`만 전달**합니다. 내려받기가 막히면 `dist\_download\jre.zip`, `dist\_download\mysql.zip`으로 직접 저장한 뒤 다시 실행합니다.
4. 백엔드 코드가 바뀌면 `make-bundle.bat`을 다시 실행합니다.

### C. 받은 사람이 하는 일
1. 압축을 풉니다. (한글 경로면 자동으로 가상 드라이브에 연결합니다. 그래도 DB가 안 켜지면 `C:\AcceptIt` 같은 영문 경로로 옮기세요.)
2. `AcceptIt.exe` 더블클릭 → 첫 실행은 1~2분 → 브라우저에 게임이 열립니다.
3. "출처를 알 수 없는 앱" 경고는 **추가 정보 → 실행**, 방화벽 창은 **허용**.
4. 같이 하는 사람은 화면에 출력되는 **"게임 주소"**(`서버PC의IP:포트`)로 접속합니다. (포트가 5173이 아닐 수 있음)
5. 끝낼 때는 검은 창에서 `Ctrl+C` 또는 창 닫기. (DB를 안전하게 종료합니다.)
6. "VCRUNTIME / MSVCP가 없다"는 오류가 나면 [VC++ 재배포 패키지](https://aka.ms/vs/17/release/vc_redist.x64.exe)를 설치합니다.

옵션: `AcceptIt.exe -sync-only`(물품 목록만 DB에 반영), `-no-browser`(브라우저 자동 열기 끄기), `-docker`(runtime 폴더가 있어도 Docker 방식으로 실행).

### D. 여러 컴퓨터에서 같이 하기 (같은 와이파이/공유기)

모두 각자 `AcceptIt.exe`를 실행하면 됩니다. 시작 메뉴는 없고, 게임 화면의 **Host / Join**과 **방 코드**만 씁니다.

1. **방장:** 화면에서 Host로 방을 만들고 방 코드를 알려 줍니다. (검은 창에 `서버 준비 완료`가 뜬 뒤부터 Host 가능, 처음 1~2분)
2. **참가자:** 화면에서 Join → 방 코드 입력. 실행기가 같은 네트워크의 방장 컴퓨터를 자동으로 찾아 연결합니다. 참가만 할 사람은 서버 준비를 기다리지 않아도 됩니다.

동작 방식: 참가자의 실행기는 그 코드의 방이 자기 서버에 없으면 UDP 방송(포트 47321)으로 방장 컴퓨터를 찾고, 그 방에 관한 `/api/rooms/{코드}...`, `/ws/rooms/{코드}` 요청을 방장 컴퓨터의 실행기로 대신 전달합니다. 백엔드·프론트 소스는 바꾸지 않습니다. (백엔드 WebSocket 설정이 Origin `http://localhost:5173`만 허용하는 부분은 실행기가 Origin을 맞춰 줍니다.)

- 같은 와이파이/공유기여야 합니다. 학교·카페 와이파이가 기기 간 통신을 막으면 휴대폰 핫스팟을 쓰세요.
- 방장 컴퓨터 방화벽에서 허용해야 합니다. 놓쳤다면 관리자 권한 명령 프롬프트에서 `netsh advfirewall firewall add rule name="AcceptIt" dir=in action=allow protocol=TCP localport=5173`, `netsh advfirewall firewall add rule name="AcceptIt-find" dir=in action=allow protocol=UDP localport=47321` (화면 포트가 5173이 아니면 실제 게임 주소의 번호로)
- 자동으로 못 찾을 때: 참가자가 방장 창에 나온 `http://IP:포트`를 브라우저에서 직접 엽니다.
- 다른 네트워크 사이에서는 아래 E(인터넷 공개) 또는 Tailscale 같은 가상 네트워크를 쓰세요.
- 옵션: `AcceptIt.exe -menu`(시작 메뉴), `-join 192.168.0.12:5173`(주소 직접 지정), `-public`(인터넷 공개)

### E. 와이파이/네트워크가 달라도 같이 하기 (인터넷 공개)

방장 컴퓨터에서 `AcceptIt-public.bat`(또는 `AcceptIt.exe -public`)을 실행하면 [Cloudflare 임시 터널](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/do-more-with-tunnels/trycloudflare/)로 게임 화면을 `https://....trycloudflare.com` 주소로 엽니다. 참가자는 실행기 없이 브라우저(휴대폰 포함)로 그 주소를 열고 Join에 방 코드를 입력하면 됩니다.

- 주소는 검은 창과 **방장 컴퓨터 브라우저의 메인 화면(왼쪽 아래 상자)**에 표시됩니다. 복사 버튼이 있고, 같은 와이파이 주소도 함께 나옵니다. 상자는 실행기가 화면을 내려 줄 때 끼워 넣는 것이라 프론트 소스는 그대로이며, `localhost`로 연 방장 화면에만 보입니다.
- `cloudflared.exe`가 필요합니다. `make-bundle.bat`이 `runtime` 폴더에 넣어 주며, 받지 못하면 안내만 하고 같은 와이파이 모드로 계속 동작합니다.
- 주소는 실행할 때마다 바뀌고, 주소를 아는 누구나 접속할 수 있습니다. 게임이 끝나면 창을 닫으세요. 방장 컴퓨터는 켜져 있어야 합니다.
- 방화벽 설정과 같은 와이파이가 필요 없습니다.

## 물품 목록 (`items.txt`)

한 줄에 물품 하나, 형식은 `이름 | 설명 | 분류`입니다. 설명과 분류는 생략할 수 있고, `#`으로 시작하는 줄은 주석입니다.

```
전기 통근버스 100대 | 공공기관 통근버스를 전기버스로 교체합니다. 친환경 vs 초기 비용 부담 | 환경
```

- 방 설정의 물품 개수가 최대 5개이므로 **5개 이상** 적어 두세요.
- 저장한 뒤 `AcceptIt.exe`를 다시 실행하면 DB에 반영됩니다.
- 이름이 같은 물품을 두 번 적으면 오류가 납니다.
- 현재 들어 있는 31개는 예시입니다. 바꿔서 쓰세요.

## 키워드 카드 (`keywords.txt`)

찬성/반대 키워드 카드를 이 목록에서 무작위로 지급합니다. 기본으로 찬성 20개 + 반대 20개(총 40개)가 들어 있습니다.

```
찬성 | 장기적 비용 절감 | 초기에는 돈이 들어도 운영·유지 비용이 줄어 전체 비용은 오히려 낮아집니다.
반대 | 예산 낭비 | 꼭 필요하지 않은 곳에 많은 세금이 들어갑니다.
```

- 형식: `찬성|반대 | 키워드(50자 이하) | 설명`, `#`은 주석. 찬성·반대 각각 5개 이상 필요합니다.
- 실행기가 DB에 `keyword_pool` 표와 `situation_card` **INSERT 트리거**를 만듭니다. 백엔드가 카드를 저장하는 순간 같은 방·같은 물품·같은 편에서 겹치지 않는 무작위 키워드/설명으로 바꿔 넣으므로 **백엔드 코드는 그대로**입니다. (손패는 저장 이후 DB에서 다시 읽어 옵니다.)
- 반영에 실패해도 게임은 계속 실행되며, 이때 카드는 기존의 `PRO 키워드 1` 같은 이름으로 나옵니다.
- 이미 시작된 게임의 카드는 바뀌지 않고, 새로 시작하는 게임부터 적용됩니다.

## 문제 해결

| 증상 | 원인 / 해결 |
|---|---|
| 방을 만들어도 다른 창에서 Join이 안 됨, 화면 하단에 `MOCK` 바 | 가짜 서버(mock) 모드입니다. 개발 서버라면 `frontend/.env.local`에 `VITE_USE_MOCK=false`를 넣고 `npm run dev`를 다시 켭니다. `AcceptIt.exe`는 이미 실제 서버 모드입니다. |
| `Access denied for user` | DB 볼륨에 예전 계정이 남아 있습니다. Docker 방식에서는 실행기가 초기화 여부를 묻습니다. |
| 3307 / 8080 포트 충돌 | 설치 없는 모드는 자동으로 다른 포트를 씁니다. Docker 방식은 충돌하는 컨테이너를 정지할지 묻습니다. |
| `Communications link failure` | DB 컨테이너에 포트 연결이 빠졌거나 아직 준비 전입니다. 실행기가 자동으로 다시 만들고 기다립니다. |
| DB가 안 켜짐 | `data\mysqld.log`, `data\acceptit-backend.log`의 마지막 부분을 확인하세요. |

## 실행기 직접 빌드 (개발자용)

내장 프론트를 새로 만들고 exe를 다시 빌드할 때:

```powershell
cd frontend
$env:VITE_USE_MOCK = "false"
npm ci
npm run build

# 빌드 결과를 실행기에 복사
Remove-Item -Recurse -Force ..\launcher\_source\web
Copy-Item -Recurse dist ..\launcher\_source\web

# exe 빌드 (Go 1.24 이상)
cd ..\launcher\_source
go test ./...
$env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -ldflags "-s -w" -o ..\AcceptIt.exe .
```

## 검증 상태

- [x] 물품 목록 파싱, DB 반영 SQL 생성, 포트 탐색, 방장 자동 탐색(UDP) 단위 테스트 통과 (`go test ./...`)
- [x] Windows용 빌드 및 정적 검사 통과
- [x] 프론트 `/api`, `/ws` 규격과 백엔드 컨트롤러/WebSocket 이벤트 이름 일치 확인 (코드 비교)
- [ ] **Windows + Docker 환경에서의 end-to-end 실행** (실제 방 생성, 입장, 게임 진행)
- [ ] **설치 없는 모드**: 휴대용 MySQL 8.0에서 Flyway 마이그레이션(V1~V7) 정상 동작
- [x] 방 코드 기반 라우팅 / UDP 방장 탐색 / WebSocket Origin 보정 단위 테스트 통과
- [x] 키워드 목록 파싱/검증, 반영 SQL 생성 단위 테스트 통과
- [x] 메인 화면 주소 상자(banner.js): 가짜 DOM 스모크 테스트, 방장 로컬 요청만 허용하는 info API 테스트 통과
- [ ] **키워드 트리거(실제 MySQL 8.0에서 생성 및 카드 교체 동작)**
- [ ] **인터넷 공개(-public)**: cloudflared 터널로 다른 네트워크에서 접속 (코드는 주소 파싱 테스트만 통과)
- [ ] **여러 컴퓨터(같은 와이파이)에서 실제 Host/Join** (방화벽, 공유기 설정에 따라 달라질 수 있음)
- [ ] 한글 경로(가상 드라이브)에서 DB 실행
- [ ] 서로 다른 컴퓨터에서 방장 자동 탐색 및 Join (공유기 환경에 따라 UDP 브로드캐스트가 막힐 수 있음)

## 알려진 제한

- 설치 없는 모드의 DB(root 비밀번호 없음)는 `127.0.0.1`에서만 열리며 이 컴퓨터 전용입니다.
- 내장된 프론트는 빌드 시점의 스냅샷입니다. 프론트가 바뀌면 위의 "직접 빌드"로 다시 만드세요.
- `runtime/`, `dist/`, `data/`는 용량이 커서 Git에 올리지 않습니다. 배포는 `dist\AcceptIt.zip`으로 합니다.
