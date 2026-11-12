import { useEffect, useState } from 'react';
import { useGameStore } from '../store/gameStore';

/** 서버가 준 "종료 시각(endsAt)"까지 남은 초. endsAt 이 없으면 null. 서버-클라이언트 시계 차이를 보정합니다. */
export function useCountdown(endsAt: number | null | undefined): number | null {
  const offset = useGameStore((s) => s.clockOffset);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!endsAt) return;
    const t = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(t);
  }, [endsAt]);
  if (!endsAt) return null;
  return Math.max(0, Math.ceil((endsAt - (now + offset)) / 1000));
}

export const fmt = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
