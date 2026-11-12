/**
 * rest.ts — 방 만들기 / 방 입장 (REST)
 *
 * [BACKEND] 엔드포인트 명세
 *  1) POST /api/rooms                       (메인화면 "Host" 버튼)
 *     요청 : { "nickname": "홍길동" }
 *     응답 : 200 { "roomCode": "ABC123", "playerId": "p-uuid" }
 *            → 서버는 방(debate_room)을 만들고, 요청자를 방장(isHost=true) 플레이어로 등록, 6자리 영숫자 코드 발급 (중복 검사)
 *            → 이후 프론트는 /ws/rooms/{roomCode}?playerId=... 로 WebSocket 연결 (api/socket.ts)
 *
 *  2) POST /api/rooms/{roomCode}/join       (메인화면 "Join" 버튼 → 코드 입력 팝업)
 *     요청 : { "nickname": "홍길동" }
 *     응답 : 200 { "roomCode": "ABC123", "playerId": "p-uuid" }
 *     에러 : 404 { "code":"ROOM_NOT_FOUND", "message":"방을 찾을 수 없습니다." }
 *            409 { "code":"ROOM_FULL",      "message":"방이 가득 찼습니다." }      (최대 3명)
 *            409 { "code":"ALREADY_STARTED","message":"이미 시작된 방입니다." }    (LOBBY 가 아닐 때)
 *     ※ 에러 body 의 message 는 프론트가 그대로 화면에 보여줍니다. 한국어 문구로 내려주세요.
 *     ※ 입장 성공 시 방 전원에게 ROOM_STATE 브로드캐스트 (WebSocket 연결 시점에 해도 됨)
 */
import { mockServer } from './mockServer';
import { USE_MOCK } from './socket';

export interface EnterResult { roomCode: string; playerId: string }

async function post<T>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message ?? `요청에 실패했습니다. (${res.status})`);
  }
  return res.json() as Promise<T>;
}

export const createRoom = (nickname: string): Promise<EnterResult> =>
  USE_MOCK ? Promise.resolve(mockServer.createRoom(nickname)) : post('/api/rooms', { nickname });

export const joinRoom = (roomCode: string, nickname: string): Promise<EnterResult> =>
  USE_MOCK
    ? Promise.resolve().then(() => mockServer.joinRoom(roomCode, nickname))
    : post(`/api/rooms/${encodeURIComponent(roomCode)}/join`, { nickname });
