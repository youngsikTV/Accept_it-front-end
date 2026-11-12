import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import Scene from '../components/Scene';
import { useGameStore, useMe } from '../store/gameStore';
import type { Card, ChatMessage, Stance } from '../types';

const STANCE: Record<Stance, string> = { PRO: '찬성', CON: '반대' };

function ChatLog({ messages }: { messages: ChatMessage[] }) {
  const end = useRef<HTMLDivElement>(null);
  useEffect(() => { end.current?.scrollIntoView({ block: 'nearest' }); }, [messages.length]);
  return (
    <div className="log">
      {messages.map((m) => m.kind === 'SYSTEM'
        ? <div key={m.id} className="msg sys">{m.text}</div>
        : <div key={m.id} className="msg">
            <b className={m.stance === 'PRO' ? 'pro' : 'con'}>[{m.stance ? STANCE[m.stance] : ''}{m.kind === 'REBUTTAL' ? ' 반론' : ''}]</b> {m.nickname}: {m.text}
            {m.keyword && <i className="kw"> #{m.keyword}</i>}
          </div>)}
      <div ref={end} />
    </div>
  );
}

function KeywordHand({ hand, selected, onSelect, enabled }: { hand: Card[]; selected: number | null; onSelect: (id: number | null) => void; enabled: boolean }) {
  if (!hand.length) return <small>사회자는 키워드 카드를 받지 않습니다.</small>;
  return (
    <div className="hand">
      <small>내 키워드 카드 (발언/반론 때 1장 사용)</small>
      {hand.map((c) => (
        <button key={c.cardId} title={c.description} disabled={c.used || !enabled}
          className={`hand-card ${selected === c.cardId ? 'sel' : ''} ${c.used ? 'used' : ''}`}
          onClick={() => onSelect(selected === c.cardId ? null : c.cardId)}>
          {c.keyword}
        </button>
      ))}
    </div>
  );
}

/** 인게임 화면: 장면(Scene) + 하단 패널(키워드 카드 / 채팅 / 상황별 버튼) — 도면 이미지의 구성 그대로 */
export default function GamePage() {
  const room = useGameStore((s) => s.room);
  const hand = useGameStore((s) => s.hand);
  const messages = useGameStore((s) => s.messages);
  const myId = useGameStore((s) => s.myId);
  const send = useGameStore((s) => s.send);
  const me = useMe();
  const [card, setCard] = useState<number | null>(null);
  const [text, setText] = useState('');
  useEffect(() => setCard(null), [room?.currentItem?.index]); // 물품이 바뀌면 선택 해제
  if (!room || !me) return null;

  const isGuest = me.role === 'GUEST', isMod = me.role === 'MODERATOR';
  const iSpeak = room.speech?.speakerId === me.id;
  const pending = room.pendingRebuttal;
  const debating = room.phase === 'DEBATE';

  // 버튼 비활성 사유 (UX 힌트일 뿐 — 실제 검증은 서버가 합니다. types.ts 의 ClientEvent 주석 참고)
  const speakWhy = !debating ? '토론 중에만 가능합니다'
    : room.speech || pending ? '다른 사람이 발언 중입니다'
    : room.lastSpeakerId === me.id ? '직전 발언자는 연속으로 발언할 수 없습니다'
    : card === null ? '키워드 카드를 먼저 선택하세요' : '';
  const rebutWhy = !debating ? '토론 중에만 가능합니다'
    : !room.speech || room.speech.kind !== 'SPEECH' || iSpeak ? '반론할 상대의 발언이 없습니다'
    : pending ? '이미 반론 요청이 진행 중입니다'
    : card === null ? '키워드 카드를 먼저 선택하세요' : '';

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!text.trim() || !iSpeak) return;
    // [BACKEND] SEND_CHAT: 발언권(speech.speakerId) 보유자만 가능. 서버가 CHAT 브로드캐스트 + debate_log 저장.
    send({ type: 'SEND_CHAT', payload: { text } });
    setText('');
  };

  return (
    <div className="page game">
      <Scene room={room} messages={messages} myId={myId} />

      <div className="card panel">
        <aside className="side">
          <KeywordHand hand={hand} selected={card} onSelect={setCard} enabled={isGuest && debating} />
        </aside>

        <section className="main">
          <ChatLog messages={messages} />

          <div className="actions">
            {room.phase === 'VOTING' && isGuest && <>
              {/* [BACKEND] VOTE: VOTING 중 객원만, 1회만. 둘 다 선택하면 같은 의견→MODERATOR_DECISION, 다르면→DEBATE */}
              <button className="btn pro" disabled={me.hasVoted} onClick={() => send({ type: 'VOTE', payload: { stance: 'PRO' } })}>👍 찬성</button>
              <button className="btn con" disabled={me.hasVoted} onClick={() => send({ type: 'VOTE', payload: { stance: 'CON' } })}>👎 반대</button>
              {me.hasVoted && <small>상대 객원의 선택을 기다리는 중...</small>}
            </>}

            {debating && isGuest && <>
              {/* [BACKEND] START_SPEECH: 연속 발언 금지(lastSpeakerId), 키워드 카드 필수. 성공 시 발언권 2분 */}
              <button className="btn" disabled={!!speakWhy} title={speakWhy}
                onClick={() => { send({ type: 'START_SPEECH', payload: { cardId: card! } }); setCard(null); }}>🎤 발언</button>
              {/* [BACKEND] REQUEST_REBUTTAL: 상대 발언 중에만, 키워드 카드 필수. 사회자에게 승인 요청(pendingRebuttal) */}
              <button className="btn con" disabled={!!rebutWhy} title={rebutWhy}
                onClick={() => { send({ type: 'REQUEST_REBUTTAL', payload: { cardId: card! } }); setCard(null); }}>✋ 반론</button>
              {/* [BACKEND] END_SPEECH: 발언권자가 발언을 일찍 끝냄 */}
              {iSpeak && <button className="btn ghost" onClick={() => send({ type: 'END_SPEECH', payload: {} })}>발언 끝내기</button>}
              <small>{iSpeak ? '말씀하세요! 아래 입력창으로 의견을 보내세요.' : speakWhy || rebutWhy}</small>
            </>}

            {debating && isMod && (pending
              ? <>
                  {/* [BACKEND] DECIDE_REBUTTAL: 사회자만. approve=true→반론자 발언권 1분 / false→묵살 연출(REBUTTAL_DENIED) */}
                  <b>{room.players.find((p) => p.id === pending.requesterId)?.nickname}님의 반론 요청 [{pending.keyword}]</b>
                  <button className="btn pro" onClick={() => send({ type: 'DECIDE_REBUTTAL', payload: { approve: true } })}>반론 승인</button>
                  <button className="btn con" onClick={() => send({ type: 'DECIDE_REBUTTAL', payload: { approve: false } })}>반론 묵살</button>
                </>
              : <small>토론을 지켜보는 중... 반론 요청이 오면 여기서 승인/묵살을 결정합니다.</small>)}

            {room.phase === 'MODERATOR_DECISION' && (isMod
              ? <>
                  {/* [BACKEND] DECIDE_ITEM: 사회자만. accept=true→입건 / false→기각. 마지막 물품이면 phase=RESULT, 아니면 다음 물품 VOTING */}
                  <button className="btn pro" onClick={() => send({ type: 'DECIDE_ITEM', payload: { accept: true } })}>✅ 물품 입건(승인)</button>
                  <button className="btn con" onClick={() => send({ type: 'DECIDE_ITEM', payload: { accept: false } })}>❌ 물품 기각</button>
                </>
              : <small>사회자가 결정하는 중입니다...</small>)}

            {room.phase === 'VOTING' && isMod && <small>두 객원이 찬성/반대를 고르는 중입니다...</small>}
          </div>

          <form className="chat-input" onSubmit={submit}>
            <input value={text} disabled={!iSpeak} maxLength={200} onChange={(e) => setText(e.target.value)}
              placeholder={iSpeak ? '발언 내용을 입력하세요' : '발언권이 있을 때만 입력할 수 있어요'} />
            <button className="btn" disabled={!iSpeak || !text.trim()}>보내기</button>
          </form>
        </section>
      </div>
    </div>
  );
}
