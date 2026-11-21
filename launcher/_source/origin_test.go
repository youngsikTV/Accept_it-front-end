package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// 백엔드로 가는 /ws 요청의 Origin 이 localhost:5173 으로 바뀌는지 (startUI 와 같은 설정) 확인
func TestWSOriginRewrite(t *testing.T) {
	var got string
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Get("Origin") }))
	defer be.Close()
	target, _ := url.Parse(be.URL)
	p := newLocalProxy(target)
	srv := httptest.NewServer(p)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/ws/rooms/ABC123", nil)
	req.Header.Set("Origin", "http://192.168.0.12:5174")
	if _, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	if got != "http://localhost:5173" {
		t.Fatalf("Origin=%q", got)
	}
}
