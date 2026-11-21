package main

import "testing"

func TestParseTunnelURL(t *testing.T) {
	line := "2026-11-21T12:00:00Z INF |  https://quick-fox-demo-1234.trycloudflare.com                                  |"
	if got := parseTunnelURL(line); got != "https://quick-fox-demo-1234.trycloudflare.com" {
		t.Fatalf("got %q", got)
	}
	if parseTunnelURL("INF Requesting new quick Tunnel on trycloudflare.com...") != "" {
		t.Fatal("주소가 없는 줄에서 주소를 찾음")
	}
}
