package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInjectBanner(t *testing.T) {
	out := string(injectBanner([]byte("<html><body><div id=root></div></body></html>")))
	if !strings.Contains(out, `<script src="/__acceptit/banner.js"></script></body>`) {
		t.Fatalf("주입 위치가 이상함: %s", out)
	}
	if got := string(injectBanner([]byte("<div>x</div>"))); !strings.HasSuffix(got, "</script>") {
		t.Fatalf("body 가 없을 때: %s", got)
	}
}

func TestInfoHandlerLocalOnly(t *testing.T) {
	setTunnel("ready", "https://abc-def.trycloudflare.com")
	defer setTunnel("", "")

	req := httptest.NewRequest("GET", "/__acceptit/info", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	rec := httptest.NewRecorder()
	infoHandler(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "abc-def.trycloudflare.com") || !strings.Contains(rec.Body.String(), `"state":"ready"`) {
		t.Fatalf("로컬 요청 실패: %d %s", rec.Code, rec.Body.String())
	}

	// 인터넷 터널을 거쳐 온 요청(프록시 헤더)과 다른 컴퓨터의 요청은 거절
	for name, mod := range map[string]func(*http.Request){
		"터널": func(r *http.Request) { r.Header.Set("Cf-Connecting-Ip", "1.2.3.4") },
		"원격": func(r *http.Request) { r.RemoteAddr = "192.168.0.7:5555" },
	} {
		r := httptest.NewRequest("GET", "/__acceptit/info", nil)
		r.RemoteAddr = "127.0.0.1:5555"
		mod(r)
		rc := httptest.NewRecorder()
		infoHandler(rc, r)
		if rc.Code != 404 {
			t.Errorf("%s 요청이 거절되지 않음: %d", name, rc.Code)
		}
	}
}
