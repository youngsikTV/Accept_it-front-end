package main

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func freeUDP(t *testing.T) int {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

func TestDiscovery(t *testing.T) {
	port := freeUDP(t)
	knownCodes.Store("ABC123", struct{}{})
	go serveDiscovery(port, "5174")
	time.Sleep(200 * time.Millisecond)

	if h := discoverHosts(port, []string{"127.0.0.1"}, "", 1500*time.Millisecond); len(h) != 1 || h[0].Addr != "127.0.0.1:5174" {
		t.Fatalf("코드 없이 탐색 실패: %+v", h)
	}
	if h := discoverHosts(port, []string{"127.0.0.1"}, "abc123", 1500*time.Millisecond); len(h) != 1 {
		t.Fatalf("알려진 방 코드 탐색 실패: %+v", h)
	}
	if h := discoverHosts(port, []string{"127.0.0.1"}, "ZZZ999", 900*time.Millisecond); len(h) != 0 {
		t.Fatalf("모르는 방 코드에 응답함: %+v", h)
	}
}

func TestSnoopRoomCode(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://x/api/rooms", nil)
	body := `{"roomCode":"qwe789","playerId":"1"}`
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Request: req, Body: io.NopCloser(strings.NewReader(body))}
	if err := snoopRoomCode(resp); err != nil {
		t.Fatal(err)
	}
	if _, ok := knownCodes.Load("QWE789"); !ok {
		t.Fatal("방 코드를 읽지 못함")
	}
	after, _ := io.ReadAll(resp.Body) // 응답 내용은 그대로 전달되어야 함
	if string(after) != body {
		t.Fatalf("응답 본문이 바뀜: %s", after)
	}
}
