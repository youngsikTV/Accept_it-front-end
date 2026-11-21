package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net"
	"net/http"
	"sync"
)

//go:embed banner.js
var bannerJS []byte

// 인터넷 공개(터널) 상태: "" (안 씀) / starting / ready / failed / missing
var tunnelSt struct {
	mu         sync.Mutex
	state, url string
}

func setTunnel(state, url string) {
	tunnelSt.mu.Lock()
	tunnelSt.state, tunnelSt.url = state, url
	tunnelSt.mu.Unlock()
}

func tunnelNow() (string, string) {
	tunnelSt.mu.Lock()
	defer tunnelSt.mu.Unlock()
	return tunnelSt.state, tunnelSt.url
}

// isLocalRequest: 이 컴퓨터의 브라우저에서 온 요청만 true. (인터넷 터널을 거쳐 온 요청은 로컬 주소로 보이지만 프록시 헤더가 붙어 있음)
func isLocalRequest(r *http.Request) bool {
	if r.Header.Get("Cf-Connecting-Ip") != "" || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Forwarded-Host") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) {
		http.NotFound(w, r)
		return
	}
	state, u := tunnelNow()
	lan := []string{}
	for _, ip := range lanIPs() {
		lan = append(lan, "http://"+ip+":"+uiPort)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"state": state, "url": u, "lan": lan})
}

func bannerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(bannerJS)
}

// injectBanner: index.html 의 </body> 앞에 안내 상자 스크립트를 끼워 넣습니다. (프론트 소스는 그대로)
func injectBanner(html []byte) []byte {
	tag := []byte(`<script src="/__acceptit/banner.js"></script>`)
	if i := bytes.LastIndex(bytes.ToLower(html), []byte("</body>")); i >= 0 {
		out := append([]byte{}, html[:i]...)
		out = append(out, tag...)
		return append(out, html[i:]...)
	}
	return append(append([]byte{}, html...), tag...)
}
