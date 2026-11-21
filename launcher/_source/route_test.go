package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func body(t *testing.T, h http.Handler, method, path string, hdr map[string]string) (int, string) {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestRoomCodeFromPath(t *testing.T) {
	for in, want := range map[string]string{
		"/api/rooms/abc123/join": "ABC123", "/ws/rooms/QWE789": "QWE789", "/api/rooms": "", "/api/debate/1/state": "",
	} {
		if got := roomCodeFromPath(in); got != want {
			t.Errorf("%s → %q want %q", in, got, want)
		}
	}
}

func TestRouting(t *testing.T) {
	// 방장 컴퓨터 역할(다른 실행기의 화면 포트)
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(forwardedHeader) == "" {
			t.Error("전달된 요청에 표시 헤더가 없음")
		}
		w.Write([]byte("REMOTE"))
	}))
	defer host.Close()
	hostAddr := strings.TrimPrefix(host.URL, "http://")

	local := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("LOCAL")) })
	h := apiHandler(local)
	discoverFn = func(code string) string {
		if code == "ABC123" {
			return hostAddr
		}
		return ""
	}
	localHasFn = func(code string) bool { return code == "MINE00" }
	defer func() { discoverFn, localHasFn = discoverRoom, localHas }()

	// 서버 준비 전: 방 만들기는 503, 모르는 코드 Join 은 404, 다른 컴퓨터의 방 Join 은 전달
	localReady.Store(false)
	if c, _ := body(t, h, "POST", "/api/rooms", nil); c != 503 {
		t.Errorf("준비 전 방 만들기: %d", c)
	}
	if c, _ := body(t, h, "POST", "/api/rooms/ZZZ999/join", nil); c != 404 {
		t.Errorf("모르는 코드: %d", c)
	}
	if c, b := body(t, h, "POST", "/api/rooms/abc123/join", nil); c != 200 || b != "REMOTE" {
		t.Errorf("원격 방 Join: %d %s", c, b)
	}
	// 이후 같은 방 요청(WebSocket 경로 포함)은 방장 컴퓨터로 계속 전달
	if _, b := body(t, h, "GET", "/ws/rooms/ABC123?playerId=2", nil); b != "REMOTE" {
		t.Errorf("원격 방 후속 요청: %s", b)
	}
	// 서버 준비 후: 방 만들기와 내 방은 로컬
	localReady.Store(true)
	defer localReady.Store(false)
	if _, b := body(t, h, "POST", "/api/rooms", nil); b != "LOCAL" {
		t.Errorf("방 만들기: %s", b)
	}
	if _, b := body(t, h, "POST", "/api/rooms/MINE00/join", nil); b != "LOCAL" {
		t.Errorf("내 방: %s", b)
	}
	// 다른 컴퓨터가 전달해 온 요청은 다시 찾지 않고 로컬 처리
	if _, b := body(t, h, "POST", "/api/rooms/ABC123/join", map[string]string{forwardedHeader: "1"}); b != "LOCAL" {
		t.Errorf("전달된 요청: %s", b)
	}
}

func TestDiscoverFirstWithCode(t *testing.T) {
	port := freeUDP(t)
	knownCodes.Store("HOST01", struct{}{})
	go serveDiscovery(port, "5199")
	time.Sleep(200 * time.Millisecond)
	if got := discoverFirst("host01", port, []string{"127.0.0.1"}, 500*time.Millisecond, 2); got != "127.0.0.1:5199" {
		t.Fatalf("방장 탐색 실패: %q", got)
	}
	if got := discoverFirst("NOPE00", port, []string{"127.0.0.1"}, 300*time.Millisecond, 1); got != "" {
		t.Fatalf("없는 코드에 응답함: %q", got)
	}
}
