/**
 * ============================================================================
 *  types.ts  —  프론트 ↔ 백엔드 "데이터 계약(Contract)"
 * ============================================================================
 * [BACKEND] 이 파일이 JSON 필드명/값의 기준입니다. 백엔드 DTO(Java record)를 이 타입과 1:1로 맞춰주세요.
 *
 *  1) 통신 방식
 *     - 방 만들기 / 방 입장        : REST        (api/rest.ts)
 *     - 그 이후 모든 실시간 통신   : WebSocket   (api/socket.ts)  메시지 형식 = { "type": string, "payload": object }
 *
 *  2) 서버가 "진실의 원천(Single Source of Truth)"
 *     - 프론트는 규칙을 판단하지 않고, 서버가 보낸 ROOM_STATE 를 그대로 그립니다.
 *     - 프론트의 버튼 비활성화는 UX 힌트일 뿐이므로, 모든 규칙 검증은 반드시 서버에서 다시 하세요.
 *       (위반 시 ERROR 이벤트로 code/message 응답 → 프론트가 토스트로 표시)
 *
 *  3) 시간 값은 전부 epoch milliseconds(number). 타이머는 "남은 시간"이 아니라 "종료 시각(endsAt)"을 내려주세요.
 *     프론트는 ROOM_STATE.serverTime 과 자기 시계의 차이를 보정해서 카운트다운을 그립니다.
 *
 *  4) ROOM_STATE 는 "수신자별"로 만들어 보내야 합니다 (개인화).
 *     - VOTING 단계에서는 상대 객원의 stance 를 null 로 가리고 hasVoted 만 알려주세요.
 *       (내 stance 는 보임. 두 객원이 모두 선택해 VOTING 이 끝나면 전원에게 공개)
 *     - 키워드 카드 손패(HAND)는 본인에게만 전송합니다.
 *
 *  5) 변경이 생길 때마다 ROOM_STATE "전체 스냅샷"을 방 전원에게 브로드캐스트하는 방식을 권장합니다.
 *     (diff 방식보다 구현이 단순하고, 재접속/새로고침 때도 같은 코드로 복구 가능)
 *     채팅 로그만 예외로 CHAT(1건) / CHAT_HISTORY(재접속 시 전체)로 따로 보냅니다.
 * ============================================================================
 */

// ----- 게임 상수 -------------------------------------------------------------
// [BACKEND] 아래 값의 "실제 기준"은 서버입니다. 프론트는 슬라이더 범위/안내 문구에만 사용합니다.
export const REQUIRED_PLAYERS = 3; // 사회자 1 + 객원 2. 3명이 안 모이면 NOT_ENOUGH_PLAYERS
export const SPEECH_SECONDS = 120; // 발언권 2분
export const REBUTTAL_SECONDS = 60; // 반론 승인 시 발언권 1분
export const SETTINGS_LIMITS = {
  maxItems: { min: 1, max: 5 }, // 최대 물품 수 (기획: 5개)
  itemSeconds: { min: 180, max: 1200, step: 60 }, // 물품 1개당 토론 시간: 3분 ~ 20분
  cardsPerItem: { min: 1, max: 5 }, // 물품 1개당 객원 1명에게 주는 키워드 카드 수
} as const;

// ----- 기본 타입 -------------------------------------------------------------
export type Role = 'NONE' | 'GUEST' | 'MODERATOR'; // NONE = 아직 역할 칸으로 이동 안 함 (방 안에서 GUEST 2명 / MODERATOR 1명만 가능)
export type Stance = 'PRO' | 'CON'; // 찬성 / 반대 (물품마다 새로 선택)

/**
 * 게임 진행 단계. 프론트는 이 값으로 페이지를 전환합니다.
 *  LOBBY              → /room    방 설정, 역할 선택, 준비
 *  VOTING             → /game    AI 물품 공개, 두 객원이 찬성/반대 선택
 *                                (두 객원 의견이 같으면 곧바로 MODERATOR_DECISION 으로 점프 = 규칙 2-1)
 *  DEBATE             → /game    발언/반론 진행. itemEndsAt 이 지나면 서버가 MODERATOR_DECISION 으로 전환
 *  MODERATOR_DECISION → /game    사회자가 물품 입건(승인) / 기각 결정
 *  RESULT             → /result  최대 물품 수만큼 끝난 뒤 결과
 */
export type Phase = 'LOBBY' | 'VOTING' | 'DEBATE' | 'MODERATOR_DECISION' | 'RESULT';

export interface Player {
  id: string; // 서버가 발급한 플레이어 ID (REST 응답의 playerId 와 동일)
  nickname: string;
  role: Role;
  ready: boolean; // LOBBY 의 READY 버튼 상태
  stance: Stance | null; // 현재 물품에 대한 찬반. VOTING 중 타인의 값은 null 로 마스킹 (위 4번 참고)
  hasVoted: boolean; // 찬/반 선택 완료 여부 (마스킹과 무관하게 항상 정확히)
  isHost: boolean; // 방장 = 방 설정(UPDATE_SETTINGS) 변경 권한자
}

export interface RoomSettings {
  maxItems: number; // 진행할 물품 수 (1~5)
  itemSeconds: number; // 물품 1개당 토론 시간(초). 180~1200
  cardsPerItem: number; // 물품 1개당 객원에게 주는 키워드 카드 수 (1~5)
}

export interface Item {
  index: number; // 1부터 시작하는 현재 물품 순번 (화면에 "물품 2/5"로 표시)
  name: string; // AI가 낸 물품 이름
  description: string; // 물품 설명/쟁점 (화이트보드에 표시)
}

/** 현재 발언권을 가진 사람. 없으면 null */
export interface Speech {
  speakerId: string;
  kind: 'SPEECH' | 'REBUTTAL'; // 일반 발언(2분) / 승인된 반론 발언(1분)
  endsAt: number; // 발언권 종료 시각. 지나면 서버가 speech=null 처리
  keyword: string; // 이 발언에서 제시한 키워드 (매 발언마다 키워드 필수)
}

/** 사회자의 승인/묵살을 기다리는 반론 요청. 없으면 null */
export interface PendingRebuttal {
  requesterId: string; // 반론을 누른 객원
  targetId: string; // 반론 대상(현재 발언자)
  keyword: string; // 반론과 함께 제시한 키워드
}

/**
 * 일회성 연출 트리거. 서버는 이벤트가 발생할 때마다 id 를 증가시켜 ROOM_STATE 에 실어주세요.
 * 프론트는 "id 가 바뀌었을 때만" 애니메이션을 한 번 재생합니다. (재접속 시 같은 id 는 재생 안 함)
 *  REBUTTAL_DENIED   : 묵살 → 사회자가 targetId 를 후려치는 모션
 *  REBUTTAL_APPROVED : 반론 승인 → targetId(반론자) 강조
 *  ITEM_ACCEPTED/REJECTED : 물품 입건/기각 배너
 */
export type EffectType = 'REBUTTAL_APPROVED' | 'REBUTTAL_DENIED' | 'ITEM_ACCEPTED' | 'ITEM_REJECTED';
export interface Effect {
  id: number;
  type: EffectType;
  targetId?: string;
}

/** 물품별 최종 결과. 결과 화면은 accepted=true 개수만 보여줍니다. */
export interface ItemResult {
  itemName: string;
  accepted: boolean;
}

/** 방 전체 상태 스냅샷 (서버 → 클라이언트 ROOM_STATE) */
export interface RoomState {
  roomCode: string; // 입장 코드 (예: ABC123)
  serverTime: number; // 이 스냅샷을 만든 서버 시각 (시계 보정용)
  phase: Phase;
  players: Player[]; // 최대 3명
  settings: RoomSettings;
  currentItem: Item | null; // LOBBY/RESULT 에서는 null
  itemEndsAt: number | null; // DEBATE 단계에서만 값이 있음 (물품 전체 토론 종료 시각)
  speech: Speech | null;
  lastSpeakerId: string | null; // "직전 발언자" — 이 사람은 발언 버튼을 연속으로 누를 수 없음 (규칙)
  pendingRebuttal: PendingRebuttal | null;
  effect: Effect | null;
  results: ItemResult[]; // 지금까지 끝난 물품 결과 (RESULT 에서 최종)
}

/** 키워드 카드 (DB: situation_card). 본인 손패로만 전송 (HAND 이벤트) */
export interface Card {
  cardId: number; // situation_card.card_id
  keyword: string; // situation_card.keyword  ← 발언/반론 때 제시되는 값
  title: string; // situation_card.title
  description: string; // situation_card.description (카드에 마우스를 올리면 표시)
  used: boolean; // 이미 사용한 카드. 사용 즉시 서버가 true 로 바꿔 HAND 재전송
}

export type ChatKind = 'SPEECH' | 'REBUTTAL' | 'SYSTEM';
/** 채팅/로그 한 줄 (DB: debate_log 에 저장하면 좋음) */
export interface ChatMessage {
  id: string;
  kind: ChatKind; // SYSTEM = "OO님이 선택을 마쳤습니다" 같은 안내 문구 (senderId 없음)
  senderId?: string;
  nickname?: string;
  stance?: Stance; // 보낸 사람의 찬반
  text: string;
  keyword?: string; // 이 발언에서 제시된 키워드
  createdAt: number;
}

// ----- 클라이언트 → 서버 이벤트 ---------------------------------------------
/**
 * [BACKEND] 아래 각 이벤트마다 "서버가 반드시 검증해야 할 규칙"을 적어두었습니다.
 *  검증 실패 시 ROOM_STATE 는 보내지 말고 요청자에게만 ERROR { code, message } 를 보내주세요.
 */
export type ClientEvent =
  /** LOBBY 에서 방장만. 값은 SETTINGS_LIMITS 범위로 검증. 설정이 바뀌면 모든 ready 를 해제하는 것을 권장 (code: NOT_HOST) */
  | { type: 'UPDATE_SETTINGS'; payload: RoomSettings }
  /** LOBBY 에서만. MODERATOR 는 1자리, GUEST 는 2자리. 가득 차면 ROLE_FULL. 역할을 바꾸면 ready 해제 */
  | { type: 'SET_ROLE'; payload: { role: Role } }
  /** LOBBY 에서만. ready=true 는 역할이 NONE 이면 ROLE_REQUIRED, 인원이 3명 미만이면 NOT_ENOUGH_PLAYERS.
   *  3명 전원 ready → 서버가 AI로 물품 생성 → 카드 지급 → phase=VOTING 으로 전환 (게임 시작) */
  | { type: 'SET_READY'; payload: { ready: boolean } }
  /** VOTING 에서 GUEST 만, 1회만(ALREADY_VOTED). 두 객원이 모두 선택하면:
   *    같은 의견 → 즉시 MODERATOR_DECISION (규칙 2-1)
   *    다른 의견 → DEBATE 시작, itemEndsAt = now + itemSeconds */
  | { type: 'VOTE'; payload: { stance: Stance } }
  /** DEBATE 에서 GUEST 만. 검증:
   *    - speech 가 null 이고 pendingRebuttal 도 null (FLOOR_BUSY)
   *    - lastSpeakerId !== 요청자 (CONSECUTIVE_SPEECH)  ← "전 턴에 발언 누른 경우 못 누름"
   *    - cardId 가 내 손패의 미사용 카드 (CARD_REQUIRED) → 사용 처리(used=true)
   *  성공 시 speech = { speakerId, kind:'SPEECH', endsAt: now+120s, keyword }, lastSpeakerId = 요청자 */
  | { type: 'START_SPEECH'; payload: { cardId: number } }
  /** 발언권(speech.speakerId)을 가진 사람만 (NOT_YOUR_TURN). 200자 제한 권장.
   *  speech.kind 에 따라 SPEECH/REBUTTAL 로 CHAT 브로드캐스트 + debate_log 저장 */
  | { type: 'SEND_CHAT'; payload: { text: string } }
  /** 발언권자가 직접 발언을 끝냄. speech=null */
  | { type: 'END_SPEECH'; payload: Record<string, never> }
  /** DEBATE 에서 GUEST 만. 검증:
   *    - 현재 speech.kind==='SPEECH' 이고 speaker 가 본인이 아님 (NO_TARGET)
   *    - pendingRebuttal 이 null (PENDING)
   *    - 미사용 카드 (CARD_REQUIRED) → 사용 처리
   *  성공 시 pendingRebuttal = { requesterId, targetId: 현재 발언자, keyword }  (사회자에게 승인 요청)
   *  ※ 사회자가 결정하는 동안 기존 발언 타이머를 멈출지/계속 흘릴지는 기획 확인 필요 (현재 mock = 계속 흐름) */
  | { type: 'REQUEST_REBUTTAL'; payload: { cardId: number } }
  /** MODERATOR 만, pendingRebuttal 이 있을 때만.
   *  approve=true  → speech = { speakerId: requesterId, kind:'REBUTTAL', endsAt: now+60s }, lastSpeakerId=requesterId, effect=REBUTTAL_APPROVED
   *  approve=false → pendingRebuttal=null, effect=REBUTTAL_DENIED(targetId=requesterId)  ("묵살" 모션) */
  | { type: 'DECIDE_REBUTTAL'; payload: { approve: boolean } }
  /** MODERATOR 만, phase===MODERATOR_DECISION 일 때만. results 에 추가 + effect=ITEM_ACCEPTED/REJECTED.
   *  currentItem.index < maxItems 이면 다음 물품(VOTING) / 아니면 phase=RESULT */
  | { type: 'DECIDE_ITEM'; payload: { accept: boolean } };

// ----- 서버 → 클라이언트 이벤트 ---------------------------------------------
export type ServerEvent =
  /** 방 전체 스냅샷. WebSocket 연결 직후에도 반드시 1회 전송 (새로고침/재접속 복구용) */
  | { type: 'ROOM_STATE'; payload: RoomState }
  /** 내 키워드 카드 손패 (연결 직후 + 물품 시작 + 카드 사용 시) */
  | { type: 'HAND'; payload: Card[] }
  /** 새 채팅/로그 1건 */
  | { type: 'CHAT'; payload: ChatMessage }
  /** 현재 물품의 채팅 로그 전체 (연결 직후, 물품이 바뀔 때는 빈 배열로 초기화) */
  | { type: 'CHAT_HISTORY'; payload: ChatMessage[] }
  /**
   * 요청 실패. code 목록(프론트는 message 를 그대로 토스트로 보여줌):
   *  NOT_HOST, ROLE_FULL, ROLE_REQUIRED, NOT_ENOUGH_PLAYERS, ALREADY_VOTED, FLOOR_BUSY, CONSECUTIVE_SPEECH,
   *  CARD_REQUIRED, NOT_YOUR_TURN, NO_TARGET, PENDING, INVALID,
   *  ROOM_NOT_FOUND ← 이 코드를 받으면 프론트는 세션을 지우고 메인으로 돌아갑니다.
   */
  | { type: 'ERROR'; payload: { code: string; message: string } };
