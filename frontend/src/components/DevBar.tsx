import { USE_MOCK } from '../api/socket';
import { useGameStore } from '../store/gameStore';

const ROLE: Record<string, string> = { NONE: '대기', GUEST: '객원', MODERATOR: '사회자' };

/** [MOCK ONLY] 한 브라우저에서 3명 시점을 번갈아 테스트하는 도구. VITE_USE_MOCK=false 면 나타나지 않습니다. */
export default function DevBar() {
  const { room, myId, roomCode, enter } = useGameStore();
  if (!USE_MOCK || !room) return null;
  return (
    <div className="devbar">
      🧪 MOCK 시점 전환:
      {room.players.map((p) => (
        <button key={p.id} className={p.id === myId ? 'on' : ''} onClick={() => enter(roomCode, p.id)}>
          {p.nickname}({ROLE[p.role]})
        </button>
      ))}
    </div>
  );
}
