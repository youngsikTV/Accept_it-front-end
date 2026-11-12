import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { createRoom, joinRoom, type EnterResult } from '../api/rest';
import { useGameStore } from '../store/gameStore';

/** 메인화면: Host / Join. (게임 설명 버튼은 App.tsx 에서 우측 상단에 항상 표시) */
export default function MainPage() {
  const nav = useNavigate();
  const enter = useGameStore((s) => s.enter);
  const [nickname, setNickname] = useState('');
  const [joinOpen, setJoinOpen] = useState(false);
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');

  const go = async (call: () => Promise<EnterResult>) => {
    if (!nickname.trim()) return setErr('닉네임을 입력하세요.');
    setBusy(true); setErr('');
    try {
      const r = await call(); // [BACKEND] REST 호출: api/rest.ts 의 명세 참고
      enter(r.roomCode, r.playerId); // 이어서 WebSocket 연결
      nav('/room');
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  };

  return (
    <div className="page hero">
      <img src="/assets/characters/bird_front.png" alt="" />
      <img src="/assets/characters/squirrel_front.png" alt="" />
      <h1>ACCEPT IT</h1>
      <p>조달청 물품 입건 토론 — 우리 세금으로 살 물건, 입건할까 기각할까?</p>
      <div className="menu">
        <input className="text" placeholder="닉네임" maxLength={10} value={nickname} onChange={(e) => setNickname(e.target.value)} />
        <button className="btn big" disabled={busy} onClick={() => go(() => createRoom(nickname.trim()))}>Host (방 만들기)</button>
        <button className="btn big ghost" disabled={busy} onClick={() => (nickname.trim() ? setJoinOpen(true) : setErr('닉네임을 입력하세요.'))}>Join (코드로 입장)</button>
        {err && <div className="err">{err}</div>}
      </div>

      {joinOpen && (
        <div className="overlay" onClick={() => setJoinOpen(false)}>
          <div className="card modal" onClick={(e) => e.stopPropagation()}>
            <h3>방 코드를 입력하세요</h3>
            <input className="text" autoFocus placeholder="예: ABC123" maxLength={8} value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} />
            <div className="row">
              <button className="btn" disabled={busy || !code} onClick={() => go(() => joinRoom(code, nickname.trim()))}>입장</button>
              <button className="btn ghost" onClick={() => setJoinOpen(false)}>취소</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
