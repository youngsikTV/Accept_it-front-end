import { useNavigate } from 'react-router-dom';
import { useGameStore } from '../store/gameStore';

/** 최종 승인 창: 각자의 의견에 따라 결정된(입건된) 물품 개수만 보여줍니다. */
export default function ResultPage() {
  const nav = useNavigate();
  const room = useGameStore((s) => s.room);
  const leave = useGameStore((s) => s.leave);
  if (!room) return null;
  const accepted = room.results.filter((r) => r.accepted).length;
  return (
    <div className="page card result">
      <h2>최종 입건 결과</h2>
      <div className="big-number">{accepted}<small> / {room.results.length}개</small></div>
      <p>우리 방이 입건한 물품 개수입니다.</p>
      <div className="row">
        {/* 기획서: "일단 넘어가기 버튼은 다음" → 다음 단계가 정해질 때까지 비활성 자리표시자 */}
        <button className="btn" disabled>넘어가기 (준비 중)</button>
        <button className="btn ghost" onClick={() => { leave(); nav('/'); }}>처음으로</button>
      </div>
    </div>
  );
}
