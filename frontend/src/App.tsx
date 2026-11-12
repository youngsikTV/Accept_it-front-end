import { useEffect } from 'react';
import { BrowserRouter, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import DevBar from './components/DevBar';
import { RulesButton } from './components/RulesModal';
import GamePage from './pages/GamePage';
import MainPage from './pages/MainPage';
import ResultPage from './pages/ResultPage';
import RoomPage from './pages/RoomPage';
import { useGameStore } from './store/gameStore';
import type { Phase } from './types';

const PATH: Record<Phase, string> = { LOBBY: '/room', VOTING: '/game', DEBATE: '/game', MODERATOR_DECISION: '/game', RESULT: '/result' };

function Shell() {
  const { pathname } = useLocation();
  const nav = useNavigate();
  const room = useGameStore((s) => s.room);
  const roomCode = useGameStore((s) => s.roomCode);
  const error = useGameStore((s) => s.error);
  const clearError = useGameStore((s) => s.clearError);

  // 서버가 알려준 phase 에 맞춰 페이지를 자동 전환 (프론트가 직접 화면을 넘기지 않음)
  useEffect(() => {
    if (!room) { if (!roomCode && pathname !== '/') nav('/'); return; }
    const target = PATH[room.phase];
    if (pathname !== target) nav(target);
  }, [room?.phase, roomCode, pathname]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!error) return;
    const t = window.setTimeout(clearError, 3500);
    return () => window.clearTimeout(t);
  }, [error, clearError]);

  return (
    <div className="app">
      {/* 게임 설명 버튼은 인게임을 제외하고 항상 우측 상단 (인게임은 Scene 안에 별도 아이콘) */}
      {pathname !== '/game' && <RulesButton className="top-right" />}
      <Routes>
        <Route path="/" element={<MainPage />} />
        <Route path="/room" element={<RoomPage />} />
        <Route path="/game" element={<GamePage />} />
        <Route path="/result" element={<ResultPage />} />
      </Routes>
      {error && <div className="toast" onClick={clearError}>⚠ {error}</div>}
      <DevBar />
    </div>
  );
}

export default function App() {
  return <BrowserRouter><Shell /></BrowserRouter>;
}
