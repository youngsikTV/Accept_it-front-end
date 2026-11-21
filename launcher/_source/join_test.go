package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAddr(t *testing.T) {
	cases := map[string]string{
		"192.168.0.12":               "192.168.0.12:5173",
		"192.168.0.12:5174":          "192.168.0.12:5174",
		" http://192.168.0.12:5174/": "192.168.0.12:5174",
		"":                           "",
	}
	for in, want := range cases {
		if got := normalizeAddr(in); got != want {
			t.Errorf("normalizeAddr(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRunJoinConnects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer srv.Close()
	stdin = bufioReader("\n") // 마지막 'Enter' 대기를 통과
	runJoin(strings.TrimPrefix(srv.URL, "http://"), true)
}

func bufioReader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }
