import { create } from 'zustand';
import { transport, USE_MOCK } from '../api/socket';
import type { Card, ChatMessage, ClientEvent, RoomState } from '../types';

/**
 * 전역 게임 상태. 서버가 보낸 값을 그대로 보관하고, 화면은 이 값만 읽어서 그립니다.
 * (규칙 판단은 서버, 화면은 표시만 — types.ts 상단 설명 참고)
 */
interface GameStore {
  myId: string;
  roomCode: string;
  room: RoomState | null;
  hand: Card[];
  messages: ChatMessage[];
  error: string | null;
  clockOffset: number; // serverTime - Date.now(). 카운트다운 시계 보정용
  enter: (roomCode: string, playerId: string) => void;
  send: (e: ClientEvent) => void;
  leave: () => void;
  clearError: () => void;
}

const SESSION_KEY = 'debate.session';

export const useGameStore = create<GameStore>((set, get) => ({
  myId: '', roomCode: '', room: null, hand: [], messages: [], error: null, clockOffset: 0,

  enter: (roomCode, playerId) => {
    sessionStorage.setItem(SESSION_KEY, JSON.stringify({ roomCode, playerId }));
    set({ roomCode, myId: playerId, room: null, hand: [], messages: [] });
    transport.open(roomCode, playerId, (e) => {
      switch (e.type) {
        case 'ROOM_STATE': set({ room: e.payload, clockOffset: e.payload.serverTime - Date.now() }); break;
        case 'HAND': set({ hand: e.payload }); break;
        case 'CHAT': set((s) => ({ messages: [...s.messages, e.payload] })); break;
        case 'CHAT_HISTORY': set({ messages: e.payload }); break;
        case 'ERROR':
          set({ error: e.payload.message });
          if (e.payload.code === 'ROOM_NOT_FOUND') get().leave();
          break;
      }
    });
  },
  send: (e) => transport.send(e),
  leave: () => {
    transport.close();
    sessionStorage.removeItem(SESSION_KEY);
    set({ myId: '', roomCode: '', room: null, hand: [], messages: [] });
  },
  clearError: () => set({ error: null }),
}));

/** 새로고침 시 이전 방에 다시 접속 (실서버 모드 전용. mock 은 메모리라서 새로고침하면 초기화됨) */
export function restoreSession() {
  if (USE_MOCK) return;
  try {
    const raw = sessionStorage.getItem(SESSION_KEY);
    if (!raw) return;
    const { roomCode, playerId } = JSON.parse(raw);
    useGameStore.getState().enter(roomCode, playerId);
  } catch { /* 무시 */ }
}

/** 내 Player 객체 */
export const useMe = () => useGameStore((s) => s.room?.players.find((p) => p.id === s.myId) ?? null);
