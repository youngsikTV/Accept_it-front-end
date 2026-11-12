# 백엔드 확인용 명세 (프론트 ↔ 서버 계약)

> 코드 안에서 `[BACKEND]` 로 검색하면 모든 확인 포인트가 나옵니다.
> 가장 정확한 기준 파일은 **`src/types.ts`** (필드/이벤트/검증 규칙), **`src/api/mockServer.ts`** (규칙을 코드로 구현한 참고본) 입니다.

## 0. 먼저 확인할 것 (build.gradle)
- `spring-boot-starter-web` 과 `spring-boot-starter-webflux` 가 **동시에** 있으면 서블릿(MVC)으로 기동되어 WebFlux 의 reactive WebSocketHandler 가 동작하지 않습니다.
  → `spring-boot-starter-websocket` 을 추가하고 `TextWebSocketHandler` 를 쓰는 것을 권장합니다. (AI 호출용 `WebClient` 는 그대로 사용 가능)

## 1. REST (`src/api/rest.ts`)
| 메서드/경로 | 요청 | 성공 | 실패 |
|---|---|---|---|
| `POST /api/rooms` | `{nickname}` | `{roomCode, playerId}` (요청자=방장) | - |
| `POST /api/rooms/{roomCode}/join` | `{nickname}` | `{roomCode, playerId}` | 404 `ROOM_NOT_FOUND`, 409 `ROOM_FULL`, 409 `ALREADY_STARTED` — body 의 `message` 는 한국어 문구로 (화면에 그대로 표시됨) |

## 2. WebSocket (`src/api/socket.ts`)
- `ws://host/ws/rooms/{roomCode}?playerId={playerId}`, JSON 텍스트 프레임 `{type, payload}`
- 연결 직후 서버는 `ROOM_STATE → HAND → CHAT_HISTORY` 를 1회씩 전송 (새로고침/재접속 복구)
- 같은 playerId 로 재접속하면 기존 플레이어로 복구. 없는 방이면 `ERROR(ROOM_NOT_FOUND)` 후 종료

### 클라이언트 → 서버
| type | payload | 서버 검증 |
|---|---|---|
| `UPDATE_SETTINGS` | `{maxItems, itemSeconds, cardsPerItem}` | 방장만, LOBBY, 범위(1~5 / 180~1200 / 1~5). 변경 시 ready 해제 권장 |
| `SET_ROLE` | `{role: NONE\|GUEST\|MODERATOR}` | LOBBY, 사회자 1·객원 2 정원(`ROLE_FULL`), 변경 시 ready 해제 |
| `SET_READY` | `{ready}` | 역할 필수(`ROLE_REQUIRED`), 3명 미만이면 `NOT_ENOUGH_PLAYERS`. 3명 전원 ready → 게임 시작 |
| `VOTE` | `{stance: PRO\|CON}` | VOTING, 객원만, 1회(`ALREADY_VOTED`). 둘 다 선택: 같으면 MODERATOR_DECISION / 다르면 DEBATE |
| `START_SPEECH` | `{cardId}` | DEBATE, 객원, 발언자 없음(`FLOOR_BUSY`), 직전 발언자 아님(`CONSECUTIVE_SPEECH`), 미사용 카드(`CARD_REQUIRED`). 발언권 2분 |
| `SEND_CHAT` | `{text}` | 발언권자만(`NOT_YOUR_TURN`), 200자 제한 |
| `END_SPEECH` | `{}` | 발언권자만 |
| `REQUEST_REBUTTAL` | `{cardId}` | DEBATE, 객원, 상대가 일반 발언 중(`NO_TARGET`), 대기 중인 요청 없음(`PENDING`), 미사용 카드 |
| `DECIDE_REBUTTAL` | `{approve}` | 사회자만, pendingRebuttal 존재. 승인=반론자 1분 / 묵살=연출 |
| `DECIDE_ITEM` | `{accept}` | 사회자만, MODERATOR_DECISION. 마지막 물품이면 RESULT |

### 서버 → 클라이언트
`ROOM_STATE`(전체 스냅샷, **수신자별 마스킹**: VOTING 중 타인의 stance=null) / `HAND`(본인 카드만) / `CHAT`(1건) / `CHAT_HISTORY`(전체) / `ERROR {code, message}`

### 서버가 직접 해야 하는 일
- **타이머**: 발언 종료(`speech.endsAt`), 물품 토론 종료(`itemEndsAt`) 시 서버가 상태를 바꾸고 ROOM_STATE 브로드캐스트. 프론트는 표시만 합니다.
- **effect**: 묵살/승인/입건/기각 때 `effect.id` 를 증가시켜 실어 보내면 프론트가 1회 연출(머리 찍기 등)을 재생합니다.
- **AI 물품 생성**: 게임 시작 시점에 호출(지연 가능). 생성이 끝난 뒤에 `phase=VOTING` 으로 전환해 주세요.

## 3. DB 스키마 보완 제안 (현재 `schema.sql` 기준)
- `debate_room` 에 컬럼 추가: `room_code`(UNIQUE), `max_items`, `item_seconds`, `cards_per_item`
- 플레이어 테이블 신설: `player(player_id, room_id, nickname, role, is_host, ready)` — `debate_log` 가 보낸 사람을 참조할 수 있어야 합니다
- 물품 테이블 신설: `debate_item(item_id, room_id, item_index, name, description, accepted)` — `results` 와 결과 화면용
- `debate_log` 보강: `player_id`, `kind`(SPEECH/REBUTTAL/SYSTEM), `keyword`, `item_id` (현재는 speaker_role 만 있어 누가 말했는지 알 수 없음)
- `situation_card` 의 `keyword/title/description` 은 프론트 `Card` 와 그대로 매핑됩니다.

## 4. 기획 확인이 필요한 가정 (mock 에서 임시로 정한 값)
1. 반론 승인 시 기존 발언자의 남은 시간은 이어가지 않고, 반론자 1분 후 발언권이 비게 됩니다.
2. 반론자도 "직전 발언자"로 취급합니다 (연속 발언 금지에 포함).
3. 반론 요청이 사회자 결정을 기다리는 동안 기존 발언 타이머는 계속 흐릅니다.
4. 찬/반 선택은 한 번 하면 바꿀 수 없고, 상대 의견은 둘 다 선택할 때까지 숨깁니다.
5. 설정(물품 수/시간/카드 수)이 바뀌면 모든 READY 가 해제됩니다.
