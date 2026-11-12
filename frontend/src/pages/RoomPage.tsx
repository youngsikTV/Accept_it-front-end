import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useGameStore, useMe } from '../store/gameStore';
import { REQUIRED_PLAYERS, SETTINGS_LIMITS, type Player, type Role, type RoomSettings } from '../types';

const ROLE_LABEL: Record<Role, string> = { NONE: '대기', GUEST: '객원', MODERATOR: '사회자' };

function Slider(props: { label: string; value: number; min: number; max: number; step?: number; show?: (v: number) => string; disabled: boolean; onChange: (v: number) => void; onCommit: () => void }) {
  const { label, value, show, onChange, onCommit, ...rest } = props;
  return (
    <label className="slider">
      <span>{label}: <b>{show ? show(value) : value}</b></span>
      <input type="range" value={value} onChange={(e) => onChange(Number(e.target.value))}
        onPointerUp={onCommit} onKeyUp={onCommit} min={rest.min} max={rest.max} step={rest.step ?? 1} disabled={rest.disabled} />
    </label>
  );
}

/** 방 설정 화면: 참가자 목록, 역할 칸 이동(드래그 또는 버튼), 물품/시간 슬라이더, READY */
export default function RoomPage() {
  const nav = useNavigate();
  const room = useGameStore((s) => s.room);
  const myId = useGameStore((s) => s.myId);
  const send = useGameStore((s) => s.send);
  const leave = useGameStore((s) => s.leave);
  const me = useMe();
  const [draft, setDraft] = useState<RoomSettings>({ maxItems: 3, itemSeconds: 300, cardsPerItem: 3 });

  // 서버 설정이 바뀌었을 때만 슬라이더 초안을 덮어씀 (드래그 중 다른 이벤트로 튀지 않도록 JSON 키로 비교)
  const key = room ? JSON.stringify(room.settings) : '';
  useEffect(() => { if (room) setDraft(room.settings); /* eslint-disable-next-line */ }, [key]);

  if (!room || !me) return <div className="page card">서버에 연결 중...</div>;

  const L = SETTINGS_LIMITS;
  const readyCount = room.players.filter((p) => p.ready).length;
  const canEdit = me.isHost;

  // [BACKEND] SET_ROLE: 사회자 1자리 / 객원 2자리. 가득 차면 ROLE_FULL. 역할을 바꾸면 ready 가 풀립니다. (types.ts 참고)
  const moveTo = (role: Role) => send({ type: 'SET_ROLE', payload: { role } });
  // [BACKEND] UPDATE_SETTINGS: 방장만, 슬라이더에서 손을 뗄 때 1회 전송. 값 범위는 서버에서도 검증.
  const commit = () => send({ type: 'UPDATE_SETTINGS', payload: draft });
  // [BACKEND] SET_READY: 역할 미선택 → ROLE_REQUIRED, 3명 미만 → NOT_ENOUGH_PLAYERS. 3명 전원 ready 면 서버가 게임 시작(phase=VOTING).
  const toggleReady = () => send({ type: 'SET_READY', payload: { ready: !me.ready } });

  const chip = (p: Player) => (
    <div key={p.id} className={`chip ${p.id === myId ? 'me' : ''}`} draggable={p.id === myId}
      onDragStart={(e) => e.dataTransfer.setData('text/plain', p.id)}>
      {p.isHost && '👑 '}{p.nickname}{p.id === myId && ' (나)'}{p.ready && ' ✅'}
    </div>
  );
  const zone = (role: Role, title: string) => (
    <div className="zone" onDragOver={(e) => e.preventDefault()}
      onDrop={(e) => { e.preventDefault(); if (e.dataTransfer.getData('text/plain') === myId) moveTo(role); }}>
      <h4>{title}</h4>
      {room.players.filter((p) => p.role === role).map(chip)}
    </div>
  );

  return (
    <div className="page card room">
      <div className="row between">
        <h2>방 설정</h2>
        <div className="row">
          <button className="btn ghost" onClick={() => navigator.clipboard?.writeText(room.roomCode)}>코드 {room.roomCode} 복사</button>
          <button className="btn ghost" onClick={() => { leave(); nav('/'); }}>나가기</button>
        </div>
      </div>

      <h3>참가자 ({room.players.length}/{REQUIRED_PLAYERS})</h3>
      <div className="pool"
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => { e.preventDefault(); if (e.dataTransfer.getData('text/plain') === myId) moveTo('NONE'); }}>
        {room.players.filter((p) => p.role === 'NONE').map(chip)}
        {room.players.every((p) => p.role !== 'NONE') && <small>모두 역할 칸에 있습니다.</small>}
      </div>
      <div className="zones">
        {zone('MODERATOR', '🔨 사회자 (1명)')}
        {zone('GUEST', '🗣 객원 (2명)')}
      </div>
      <div className="row">
        <small>내 역할 선택(드래그 대신 버튼으로도 이동):</small>
        {(['MODERATOR', 'GUEST', 'NONE'] as Role[]).map((r) => (
          <button key={r} className={`btn ${me.role === r ? '' : 'ghost'}`} onClick={() => moveTo(r)}>{ROLE_LABEL[r]}</button>
        ))}
      </div>
      {room.players.length < REQUIRED_PLAYERS && <div className="err">인원이 부족합니다. 3명이 모여야 시작할 수 있어요. (현재 {room.players.length}명)</div>}

      <h3>게임 설정 {!canEdit && <small>(방장만 변경 가능)</small>}</h3>
      <Slider label="물품 개수" value={draft.maxItems} min={L.maxItems.min} max={L.maxItems.max} disabled={!canEdit}
        onChange={(v) => setDraft({ ...draft, maxItems: v })} onCommit={commit} />
      <Slider label="물품당 토론 시간" value={draft.itemSeconds} min={L.itemSeconds.min} max={L.itemSeconds.max} step={L.itemSeconds.step}
        show={(v) => `${v / 60}분`} disabled={!canEdit} onChange={(v) => setDraft({ ...draft, itemSeconds: v })} onCommit={commit} />
      <Slider label="물품당 키워드 카드 수" value={draft.cardsPerItem} min={L.cardsPerItem.min} max={L.cardsPerItem.max} disabled={!canEdit}
        onChange={(v) => setDraft({ ...draft, cardsPerItem: v })} onCommit={commit} />

      <div className="ready-bar" style={{ position: 'fixed', right: 16, bottom: 16, display: 'flex', alignItems: 'center', gap: 10, zIndex: 40, background: '#fbfaf7', border: '4px solid #2b2a33', padding: '8px 12px', boxShadow: '4px 4px 0 rgba(0,0,0,.4)' }}>
        <span className="count" style={{ fontSize: '1.4rem', fontWeight: 700 }}>{readyCount}/{REQUIRED_PLAYERS}</span>
        <button className="btn big" onClick={toggleReady}>{me.ready ? '준비 취소' : 'READY'}</button>
      </div>
    </div>
  )
}
