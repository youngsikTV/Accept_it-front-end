/**
 * socket.ts — 실시간 통신 계층. 화면 코드는 transport 만 사용하므로 mock ↔ 실서버 전환은 이 파일만 신경 쓰면 됩니다.
 *
 * [BACKEND] WebSocket 엔드포인트 명세
 *   URL      : ws(s)://<host>/ws/rooms/{roomCode}?playerId={playerId}
 *   메시지   : 텍스트 프레임, JSON  { "type": "...", "payload": { ... } }   (타입 정의는 types.ts 의 ClientEvent / ServerEvent)
 *   연결 직후: 서버가 ROOM_STATE → HAND → CHAT_HISTORY 순서로 1회씩 전송해야 합니다. (새로고침/재접속 복구)
 *   없는 방  : ERROR { code:'ROOM_NOT_FOUND' } 전송 후 연결 종료
 *   재접속   : 프론트는 연결이 끊기면 1s→2s→4s→8s(최대) 간격으로 자동 재연결합니다. 같은 playerId 로 다시 붙으면 기존 플레이어로 복구해 주세요.
 *   보안     : 지금은 playerId 를 쿼리로 넘기는 단순 방식입니다. 시간이 되면 입장 REST 응답에 token 을 추가해 검증하세요.
 *
 * [BACKEND][주의] build.gradle 에 spring-boot-starter-web 과 spring-boot-starter-webflux 가 "동시에" 있으면
 *   Spring 은 서블릿(MVC) 방식으로 기동합니다 → WebFlux 의 reactive WebSocketHandler 는 동작하지 않습니다.
 *   해결(둘 중 하나):  ① implementation 'org.springframework.boot:spring-boot-starter-websocket' 추가 후 TextWebSocketHandler 사용 (권장, 단순)
 *                     ② starter-web 제거하고 WebFlux 로 통일
 *   AI API 호출용 WebClient 는 starter-webflux 만 있어도 MVC 앱에서 그대로 사용 가능합니다.
 */
import type { ClientEvent, ServerEvent } from '../types';
import { mockServer } from './mockServer';

/** .env 에 VITE_USE_MOCK=false 로 두면 실제 서버에 연결 */
export const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false';

type Handler = (e: ServerEvent) => void;
export interface Transport {
  open(roomCode: string, playerId: string, onEvent: Handler): void;
  send(e: ClientEvent): void;
  close(): void;
}

class MockTransport implements Transport {
  open(_roomCode: string, playerId: string, onEvent: Handler) { mockServer.connect(playerId, onEvent); }
  send(e: ClientEvent) { mockServer.send(e); }
  close() { mockServer.disconnect(); }
}

class WsTransport implements Transport {
  private ws: WebSocket | null = null;
  private retry = 0;

  open(roomCode: string, playerId: string, onEvent: Handler) {
    this.close();
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${location.host}/ws/rooms/${encodeURIComponent(roomCode)}?playerId=${encodeURIComponent(playerId)}`;
    const connect = () => {
      const ws = new WebSocket(url);
      this.ws = ws;
      ws.onopen = () => { this.retry = 0; };
      ws.onmessage = (m) => {
        try { onEvent(JSON.parse(m.data) as ServerEvent); } catch { console.error('[ws] 잘못된 메시지', m.data); }
      };
      ws.onclose = () => {
        if (this.ws !== ws) return; // 우리가 직접 닫았거나 새 연결로 교체됨
        window.setTimeout(connect, Math.min(1000 * 2 ** this.retry++, 8000));
      };
    };
    connect();
  }
  send(e: ClientEvent) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(e));
    else console.warn('[ws] 연결되지 않아 전송하지 못했습니다.', e);
  }
  close() {
    const w = this.ws; this.ws = null; w?.close();
  }
}

export const transport: Transport = USE_MOCK ? new MockTransport() : new WsTransport();
