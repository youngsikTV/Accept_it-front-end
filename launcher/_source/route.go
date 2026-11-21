package main

// 같은 네트워크 자동 연결:
//   - 모든 컴퓨터가 AcceptIt.exe 를 실행하면 각자 화면(:5173)과 서버를 켭니다.
//   - 방장은 화면의 Host 로 방을 만들고, 참가자는 Join 에 방 코드를 입력합니다.
//   - 참가자 컴퓨터의 실행기는 그 코드의 방이 자기 서버에 없으면 UDP 방송으로 같은 네트워크의 방장 컴퓨터를 찾고,
//     그 방에 관한 요청(/api/rooms/코드..., /ws/rooms/코드)을 방장 컴퓨터로 대신 전달합니다.
import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	discoveryPort   = 41234
	forwardedHeader = "X-AcceptIt-Forwarded"
)

var (
	localReady atomic.Bool // 이 컴퓨터의 DB·백엔드 준비가 끝났는지
	remoteMu   sync.Mutex
	remoteRoom = map[string]string{} // 방 코드 → 방장 컴퓨터 "IP:포트"

	discoverFn = discoverRoom // 테스트에서 교체
	localHasFn = localHas
)

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": msg})
}

// roomCodeFromPath: /api/rooms/ABC123/join, /ws/rooms/ABC123 → "ABC123"
func roomCodeFromPath(p string) string {
	for _, pre := range []string{"/api/rooms/", "/ws/rooms/"} {
		if strings.HasPrefix(p, pre) {
			rest := strings.TrimPrefix(p, pre)
			if i := strings.Index(rest, "/"); i >= 0 {
				rest = rest[:i]
			}
			return strings.ToUpper(strings.TrimSpace(rest))
		}
	}
	return ""
}

func localHas(code string) bool {
	c := http.Client{Timeout: 2 * time.Second}
	r, err := c.Get(backendURL + "/api/rooms/" + url.PathEscape(code))
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == http.StatusOK
}

func remoteFor(code string) string {
	remoteMu.Lock()
	defer remoteMu.Unlock()
	return remoteRoom[code]
}

func setRemote(code, addr string) {
	remoteMu.Lock()
	remoteRoom[code] = addr
	remoteMu.Unlock()
}

func forward(addr, code string, w http.ResponseWriter, r *http.Request) {
	rp := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: addr})
	d := rp.Director
	rp.Director = func(req *http.Request) {
		d(req)
		req.Header.Set(forwardedHeader, "1")
	}
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		remoteMu.Lock()
		delete(remoteRoom, code)
		remoteMu.Unlock()
		writeErr(w, http.StatusBadGateway, "HOST_UNREACHABLE", "방장 컴퓨터에 연결할 수 없습니다. 방장의 AcceptIt 이 켜져 있는지 확인하세요.")
	}
	rp.ServeHTTP(w, r)
}

// apiHandler: /api, /ws 요청을 이 컴퓨터의 백엔드(local) 또는 방장 컴퓨터로 보냅니다.
func apiHandler(local http.Handler) http.Handler {
	serveLocal := func(w http.ResponseWriter, r *http.Request, code string) {
		if !localReady.Load() {
			if code == "" {
				writeErr(w, http.StatusServiceUnavailable, "SERVER_STARTING", "이 컴퓨터의 서버를 준비하는 중입니다. 검은 창에 '준비 완료'가 보이면 다시 눌러 주세요. (방에 참가하는 것은 지금도 가능합니다)")
			} else {
				writeErr(w, http.StatusNotFound, "ROOM_NOT_FOUND", "방을 찾을 수 없습니다. 방장과 같은 와이파이인지, 방장의 AcceptIt 이 켜져 있는지 확인하세요.")
			}
			return
		}
		local.ServeHTTP(w, r)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := roomCodeFromPath(r.URL.Path)
		if code == "" || r.Header.Get(forwardedHeader) != "" { // 방 생성, 그 외 요청, 다른 컴퓨터가 전달해 온 요청
			serveLocal(w, r, code)
			return
		}
		if addr := remoteFor(code); addr != "" {
			forward(addr, code, w, r)
			return
		}
		if localReady.Load() && localHasFn(code) {
			serveLocal(w, r, code)
			return
		}
		if addr := discoverFn(code); addr != "" {
			setRemote(code, addr)
			forward(addr, code, w, r)
			return
		}
		serveLocal(w, r, code)
	})
}

// ---- UDP 방송으로 방장 컴퓨터 찾기 (프로토콜은 discovery.go 와 같음) ----

func discoverRoom(code string) string {
	return discoverFirst(code, discPort, broadcastTargets(), 1200*time.Millisecond, 3)
}

// discoverFirst: "ACCEPTIT?|코드" 를 방송하고, 첫 번째로 응답한 컴퓨터의 "IP:화면포트" 를 돌려줍니다.
func discoverFirst(code string, port int, targets []string, wait time.Duration, tries int) string {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return ""
	}
	defer conn.Close()
	msg := []byte("ACCEPTIT?|" + strings.ToUpper(strings.TrimSpace(code)))
	buf := make([]byte, 512)
	for i := 0; i < tries; i++ {
		for _, t := range targets {
			if a, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", t, port)); err == nil {
				_, _ = conn.WriteToUDP(msg, a)
			}
		}
		_ = conn.SetReadDeadline(time.Now().Add(wait))
		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			parts := strings.Split(strings.TrimSpace(string(buf[:n])), "|")
			if len(parts) >= 2 && parts[0] == "ACCEPTIT!" {
				return net.JoinHostPort(from.IP.String(), parts[1])
			}
		}
	}
	return ""
}
