import { useState } from 'react';

type Tab = 'GUEST' | 'MODERATOR';

const RULES: Record<Tab, { label: string; lines: string[] }> = {
  GUEST: {
    label: '토론자(객원)',
    lines: [
      'AI가 낸 물품에 대해 찬성 / 반대 버튼을 누릅니다.',
      '두 객원의 의견이 같으면 토론 없이 바로 사회자의 결정으로 넘어갑니다.',
      '의견이 갈리면 [발언] 버튼으로 2분 동안 말할 수 있습니다. 매 발언마다 키워드 카드 1장을 제시해야 합니다.',
      '직전에 발언한 사람은 연속으로 발언할 수 없습니다.',
      '상대 발언 중 키워드 카드를 골라 [반론]을 누르면 사회자에게 요청이 갑니다. 승인되면 1분의 발언권, 묵살되면 머리를 맞습니다.',
      '물품의 토론 시간이 끝나면 사회자가 입건 / 기각을 결정합니다.',
    ],
  },
  MODERATOR: {
    label: '사회자',
    lines: [
      '객원의 반론 요청이 오면 [반론 승인] 또는 [반론 묵살]을 누릅니다.',
      '두 객원의 의견이 같거나 토론 시간이 끝나면 물품을 [입건] 또는 [기각]합니다.',
      '사회자는 키워드 카드를 받지 않고, 발언도 하지 않습니다.',
      '모든 물품이 끝나면 입건된 물품 개수가 결과로 표시됩니다.',
    ],
  },
};

/** 게임 설명 버튼 + 팝업. 팝업은 역할군별 탭으로 나뉘며 기본 탭은 "토론자" (기획서 기준) */
export function RulesButton({ defaultTab = 'GUEST', className = '', icon = false }: { defaultTab?: Tab; className?: string; icon?: boolean }) {
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<Tab>(defaultTab);
  return (
    <>
      <button className={`btn ghost ${className}`} onClick={() => { setTab(defaultTab); setOpen(true); }} aria-label="게임 설명">
        {icon ? '?' : '게임 설명'}
      </button>
      {open && (
        <div className="overlay" onClick={() => setOpen(false)}>
          <div className="card modal" onClick={(e) => e.stopPropagation()}>
            <div className="tabs">
              {(Object.keys(RULES) as Tab[]).map((t) => (
                <button key={t} className={`btn ${tab === t ? '' : 'ghost'}`} onClick={() => setTab(t)}>{RULES[t].label}</button>
              ))}
            </div>
            <ol className="rules">{RULES[tab].lines.map((l) => <li key={l}>{l}</li>)}</ol>
            <button className="btn" onClick={() => setOpen(false)}>닫기</button>
          </div>
        </div>
      )}
    </>
  );
}
