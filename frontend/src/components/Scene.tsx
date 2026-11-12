import { useEffect, useRef, useState } from 'react';
import { fmt, useCountdown } from '../hooks/useCountdown';
import type { ChatMessage, Effect, Player, RoomState, Stance } from '../types';
import { RulesButton } from './RulesModal';

const IMG = '/assets/characters/';
const STANCE: Record<Stance, string> = { PRO: '찬성', CON: '반대' };
const FX_TEXT: Record<Effect['type'], string> = {
  REBUTTAL_APPROVED: '반론 승인!', REBUTTAL_DENIED: '묵살! 💢', ITEM_ACCEPTED: '입건!', ITEM_REJECTED: '기각!',
};

interface Props { room: RoomState; messages: ChatMessage[]; myId: string }

/**
 * 인게임 장면: 배경(회의실) + 테이블 + 캐릭터 3명 + 화이트보드(물품 정보) + 연출.
 * 객원 2명은 좌/우 (squirrel), 사회자는 가운데 (bird).
 * 배경을 일러스트로 교체하려면 index.css 의 .scene 에 background-image 를 지정하고 .wall/.floor/.table 을 지우면 됩니다.
 */
export default function Scene({ room, messages, myId }: Props) {
  const [left, right] = room.players.filter((p) => p.role === 'GUEST');
  const mod = room.players.find((p) => p.role === 'MODERATOR');
  const myRole = room.players.find((p) => p.id === myId)?.role;
  const itemLeft = useCountdown(room.itemEndsAt);
  const speechLeft = useCountdown(room.speech?.endsAt);

  // 서버가 effect.id 를 바꿨을 때만 연출을 1회 재생 (재접속 시 같은 id 는 무시)
  const [fx, setFx] = useState<Effect | null>(null);
  const seen = useRef(room.effect?.id ?? 0);
  useEffect(() => {
    const e = room.effect;
    if (!e || e.id === seen.current) return;
    seen.current = e.id;
    setFx(e);
    const t = window.setTimeout(() => setFx(null), 1600);
    return () => window.clearTimeout(t);
  }, [room.effect]);

  const nameOf = (id: string) => room.players.find((p) => p.id === id)?.nickname ?? '';
  const denied = fx?.type === 'REBUTTAL_DENIED';
  // 묵살 연출: 사회자가 맞는 사람 쪽으로 몸을 돌린다 (bird_left = 왼쪽을 바라봄)
  const modSprite = denied ? (fx.targetId === left?.id ? 'bird_left' : 'bird_right') : 'bird_front';

  const hint =
    room.phase === 'VOTING' ? '찬성 / 반대를 선택하세요'
    : room.phase === 'MODERATOR_DECISION' ? '사회자가 물품의 입건 여부를 결정합니다'
    : room.speech ? `${nameOf(room.speech.speakerId)}님 ${room.speech.kind === 'REBUTTAL' ? '반론' : '발언'} 중`
    : '발언 버튼을 눌러 의견을 내세요';

  const seat = (p: Player | undefined, side: 'left' | 'center' | 'right', sprite: string) => {
    if (!p) return <div className={`seat ${side}`}><div className="nameplate empty">빈 자리</div></div>;
    const speaking = room.speech?.speakerId === p.id;
    const hit = denied && fx?.targetId === p.id;
    const rebutting = room.pendingRebuttal?.requesterId === p.id;
    // ↓ 추가
    const approved = fx?.type === 'REBUTTAL_APPROVED' && fx.targetId === p.id;
    const lookAtMod = hit || rebutting || approved;          // 사회자를 바라보는 상황들
    const src = p.role === 'GUEST' && !lookAtMod ? 'squirrel_front' : sprite;
    const last = [...messages].reverse().find((m) => m.senderId === p.id);
    const status =
      p.role === 'MODERATOR' ? '🔨 사회자'
      : room.phase === 'VOTING' ? (p.hasVoted ? '선택 완료' : '고민 중...')
      : p.stance ? STANCE[p.stance] : '객원';
    return (
      <div className={`seat ${side}`}>
        {speaking && last && <div className="bubble">{last.keyword ? `[${last.keyword}] ` : ''}{last.text}</div>}
        {rebutting && <div className="bubble alert">반론!! [{room.pendingRebuttal!.keyword}]</div>}
        <img className={`sprite ${speaking ? 'speaking' : ''} ${hit ? 'hit' : ''}`} src={`${IMG}${src}.png`} alt={p.nickname} />
        <div className="monitor" /> 
        {hit && <span className="hit-star">💥</span>}
        <div className={`nameplate ${p.stance === 'PRO' ? 'pro' : p.stance === 'CON' ? 'con' : ''}`}>
          <b>{p.nickname}{p.id === myId ? ' (나)' : ''}</b>
          <div>{status}{speaking && speechLeft !== null ? ` 🎤 ${fmt(speechLeft)}` : ''}</div>
        </div>
      </div>
    );
  };

  return (
    <div className="scene">
      <div className="ceiling" /><div className="wall" /><div className="floor" />
      <div className="board">
        {room.currentItem && <>
          <small>물품 {room.currentItem.index} / {room.settings.maxItems}</small>
          <h2>{room.currentItem.name}</h2>
          <p>{room.currentItem.description}</p>
        </>}
        <em>{hint}</em>
      </div>

      {seat(left, 'left', 'squirrel_right')}
      {seat(mod, 'center', modSprite)}
      {seat(right, 'right', 'squirrel_left')}

      <div className="table" />

      <div className="hud">⏱ 물품 시간 {itemLeft !== null ? fmt(itemLeft) : '--:--'}</div>
      <RulesButton className="scene-rules" icon defaultTab={myRole === 'MODERATOR' ? 'MODERATOR' : 'GUEST'} />
      {fx && <div className="fx-banner" key={fx.id}>{FX_TEXT[fx.type]}</div>}
    </div>
  );
}
