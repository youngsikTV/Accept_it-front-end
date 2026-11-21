package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// 같은 와이파이/공유기 안에서 참가자가 방장을 자동으로 찾기 위한 간단한 UDP 탐색.
//
//	참가자 → (브로드캐스트) "ACCEPTIT?|방코드"      방코드는 비워도 됨
//	방장   → (응답)        "ACCEPTIT!|화면포트|컴퓨터이름"
//
// 방장은 자기 서버로 만들어진 방 코드만 알고 있다가, 같은 코드를 물어보는 경우에만 응답합니다.
const discPort = 47321

var knownCodes sync.Map // 이 서버에서 만들어진 방 코드

// snoopRoomCode: POST /api/rooms 응답({"roomCode":"..."})에서 방 코드를 읽어 둡니다. (응답 내용은 그대로 전달)
func snoopRoomCode(r *http.Response) error {
	if r.Request == nil || r.Request.Method != http.MethodPost || r.Request.URL.Path != "/api/rooms" ||
		r.StatusCode != http.StatusOK || r.Header.Get("Content-Encoding") != "" {
		return nil
	}
	b, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	var v struct {
		RoomCode string `json:"roomCode"`
	}
	if json.Unmarshal(b, &v) == nil && v.RoomCode != "" {
		knownCodes.Store(strings.ToUpper(v.RoomCode), struct{}{})
	}
	return nil
}

// serveDiscovery: 방장 쪽. 참가자의 탐색 요청에 응답합니다. 포트를 못 열면 조용히 포기합니다.
func serveDiscovery(port int, ui string) {
	pc, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return
	}
	name, _ := os.Hostname()
	buf := make([]byte, 512)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		parts := strings.Split(strings.TrimSpace(string(buf[:n])), "|")
		if parts[0] != "ACCEPTIT?" {
			continue
		}
		if len(parts) > 1 {
			if code := strings.ToUpper(strings.TrimSpace(parts[1])); code != "" {
				if _, ok := knownCodes.Load(code); !ok && !(localReady.Load() && localHasFn(code)) {
					continue
				}
			}
		}
		_, _ = pc.WriteTo([]byte("ACCEPTIT!|"+ui+"|"+name), from)
	}
}

type foundHost struct{ Addr, Name string }

// broadcastTargets: 255.255.255.255 와, 네트워크 어댑터별 브로드캐스트 주소
func broadcastTargets() []string {
	t := []string{"255.255.255.255"}
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || len(ipn.Mask) != net.IPv4len && len(ipn.Mask) != net.IPv6len {
				continue
			}
			ip, m := ipn.IP.To4(), ipn.Mask
			if len(m) == net.IPv6len {
				m = m[12:]
			}
			b := make(net.IP, 4)
			for i := range b {
				b[i] = ip[i] | ^m[i]
			}
			t = append(t, b.String())
		}
	}
	return t
}

// discoverHosts: 참가자 쪽. targets 로 탐색 요청을 보내고 wait 동안 응답한 방장을 모읍니다.
func discoverHosts(port int, targets []string, code string, wait time.Duration) []foundHost {
	pc, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil
	}
	defer pc.Close()
	msg := []byte("ACCEPTIT?|" + strings.ToUpper(strings.TrimSpace(code)))
	var found []foundHost
	seen := map[string]bool{}
	const tries = 3
	buf := make([]byte, 512)
	for i := 0; i < tries; i++ { // UDP 는 유실될 수 있어 여러 번 보냅니다.
		for _, t := range targets {
			if a, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", t, port)); err == nil {
				_, _ = pc.WriteTo(msg, a)
			}
		}
		_ = pc.SetReadDeadline(time.Now().Add(wait / tries))
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				break
			}
			parts := strings.Split(strings.TrimSpace(string(buf[:n])), "|")
			if len(parts) < 2 || parts[0] != "ACCEPTIT!" {
				continue
			}
			ip := from.(*net.UDPAddr).IP.String()
			addr := ip + ":" + parts[1]
			if seen[addr] {
				continue
			}
			seen[addr] = true
			name := ""
			if len(parts) > 2 {
				name = parts[2]
			}
			found = append(found, foundHost{Addr: addr, Name: name})
		}
	}
	return found
}

// runJoinAuto: 참가자 모드(자동). 방 코드로 같은 네트워크의 방장을 찾아 접속합니다.
func runJoinAuto(noOpen bool) {
	fmt.Println("  [참가자 모드] 같은 와이파이/공유기에서 방장을 자동으로 찾습니다. 이 컴퓨터에서는 서버를 켜지 않습니다.")
	code := readLine("\n방장이 알려 준 방 코드를 입력하세요 (모르면 그냥 Enter): ")
	fmt.Print("  방장을 찾는 중... ")
	hosts := discoverHosts(discPort, broadcastTargets(), code, 3*time.Second)
	fmt.Printf("%d 곳 발견\n", len(hosts))
	switch len(hosts) {
	case 0:
		fmt.Println("\n  자동으로 찾지 못했습니다. 확인하세요:")
		fmt.Println("   - 방장 컴퓨터에서 AcceptIt.exe 가 켜져 있고, 방장이 Host 로 방을 만들었나요?")
		fmt.Println("   - 방 코드를 정확히 입력했나요? (모르면 Enter 로 비워 두세요)")
		fmt.Println("   - 같은 와이파이/공유기인가요? 학교·카페 와이파이는 막혀 있을 수 있습니다 (휴대폰 핫스팟 사용)")
		fmt.Println("   - 방장 컴퓨터 방화벽에서 AcceptIt 을 허용했나요?")
		runJoin("", noOpen) // 주소 직접 입력으로 넘어감
	case 1:
		runJoin(hosts[0].Addr, noOpen)
	default:
		fmt.Println("\n  여러 방장이 발견되었습니다:")
		for i, h := range hosts {
			fmt.Printf("   %d) %s  (%s)\n", i+1, h.Addr, h.Name)
		}
		var n int
		fmt.Sscanf(readLine("번호를 고르세요: "), "%d", &n)
		if n < 1 || n > len(hosts) {
			n = 1
		}
		runJoin(hosts[n-1].Addr, noOpen)
	}
}
