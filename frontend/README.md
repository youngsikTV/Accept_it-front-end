# ACCEPT IT — 프론트엔드 (React + Vite + TypeScript)

```bash
npm install
npm run dev        # http://localhost:5173  (기본: mock 모드, 백엔드 불필요)
```

- **mock 모드(기본)**: 방을 만들면 가짜 플레이어 2명이 자동 입장합니다. 화면 좌측 하단 `MOCK 시점 전환` 버튼으로
  방장 / 민수 / 지영 시점을 번갈아 눌러 3명 역할을 혼자 테스트할 수 있습니다.
  (테스트 순서: 역할 분배 → 3명 모두 READY → 찬/반 → 발언·반론 → 승인/묵살 → 입건/기각 → 결과)
- **실서버 연결**: `.env` 파일에 `VITE_USE_MOCK=false` → Spring(8080) 으로 `/api`, `/ws` 프록시. 명세는 `BACKEND_CONTRACT.md`.
- 캐릭터 이미지: `public/assets/characters/` (bird_*, squirrel_*). 배경은 CSS로 그린 임시 회의실이며 일러스트가 나오면 `.scene` 에 교체.

폴더: `pages/`(Main, Room, Game, Result) · `components/`(Scene, RulesModal, DevBar) · `api/`(rest, socket, mockServer) · `store/gameStore.ts` · `types.ts`(서버 계약)
