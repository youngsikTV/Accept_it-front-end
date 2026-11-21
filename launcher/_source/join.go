package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var stdin = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	s, _ := stdin.ReadString('\n')
	return strings.TrimSpace(s)
}

// lanIPs: 이 컴퓨터의 사설 IPv4 주소들 (같은 공유기/와이파이의 다른 컴퓨터가 접속할 때 쓰는 주소)
func lanIPs() []string {
	var out []string
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || ip[0] == 169 { // 169.254.x.x(자동 할당)은 제외
				continue
			}
			out = append(out, ip.String())
		}
	}
	return out
}

func openBrowser(u string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		_ = exec.Command("open", u).Start()
	}
}

// normalizeAddr: "192.168.0.12" / "192.168.0.12:5174" / "http://192.168.0.12:5174/" → "192.168.0.12:5174"
func normalizeAddr(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "http://"), "https://")
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if !strings.Contains(s, ":") {
		s += ":" + "5173"
	}
	return s
}

// runJoin: 참가자 모드. 방장 컴퓨터의 주소에 접속되는지 확인하고 브라우저를 엽니다. (이 컴퓨터에서는 아무 서버도 켜지 않음)
func runJoin(arg string, noOpen bool) {
	fmt.Println("  [참가자 모드] 방장 컴퓨터의 '게임 주소'로 접속합니다. 이 컴퓨터에서는 서버를 켜지 않습니다.")
	addr := normalizeAddr(arg)
	for {
		if addr == "" {
			addr = normalizeAddr(readLine("\n방장 컴퓨터의 게임 주소를 입력하세요 (예: 192.168.0.12:5173, 그냥 Enter 면 종료): "))
			if addr == "" {
				return
			}
		}
		u := "http://" + addr
		fmt.Printf("  %s 에 연결을 확인하는 중...\n", u)
		c := http.Client{Timeout: 5 * time.Second}
		r, err := c.Get(u + "/")
		if err == nil && r.StatusCode == 200 {
			r.Body.Close()
			fmt.Println("  연결되었습니다!")
			if !noOpen {
				openBrowser(u)
			}
			fmt.Printf("\n브라우저가 열리지 않으면 직접 열어 주세요: %s\n게임 화면에서 Join 을 눌러 방장이 알려 준 방 코드를 입력하세요.\n", u)
			readLine("\n이 창은 닫아도 됩니다. (Enter 를 누르면 종료)")
			return
		}
		if r != nil {
			r.Body.Close()
		}
		fmt.Println("\n  [접속 실패] 아래를 확인하세요.")
		fmt.Println("   1) 방장 컴퓨터에서 AcceptIt.exe 가 켜져 있나요? (방장 창에 '게임 주소'가 보여야 합니다)")
		fmt.Println("   2) 주소와 포트가 방장 창에 나온 것과 같나요? (포트가 5173 이 아닐 수 있습니다)")
		fmt.Println("   3) 같은 와이파이/공유기에 연결되어 있나요? (학교·카페 와이파이는 막혀 있을 수 있음 → 휴대폰 핫스팟 사용)")
		fmt.Println("   4) 방장 컴퓨터의 방화벽에서 AcceptIt 이 허용되어 있나요? (사용법.txt 참고)")
		addr = ""
	}
}

// chooseRole: 실행할 때 방장/참가자를 고릅니다. 기본값(Enter)은 방장입니다.
func chooseRole() string {
	fmt.Println("\n어떻게 시작할까요?")
	fmt.Println("  1) 방장 (이 컴퓨터가 서버가 됩니다. 한 명만 선택하세요)  ← 그냥 Enter")
	fmt.Println("  2) 참가자 (같은 와이파이의 방장을 방 코드로 자동으로 찾아 접속)")
	fmt.Println("  3) 참가자 (방장 컴퓨터의 주소를 직접 입력)")
	switch readLine("선택 (1/2/3): ") {
	case "2":
		return "join-auto"
	case "3":
		return "join"
	default:
		return "host"
	}
}

func printShareInfo() {
	fmt.Println("\n[같이 하는 방법]")
	fmt.Println("  방장: 화면에서 Host 로 방을 만들고 방 코드를 알려 주세요.")
	fmt.Println("  참가자: 각자 AcceptIt.exe 를 실행하고, 화면에서 Join 에 방 코드를 입력하면 같은 와이파이의 방장을 자동으로 찾습니다.")
	fmt.Println("  ※ 같은 와이파이/공유기여야 하며, 첫 실행 때 뜨는 방화벽 창은 '허용'을 눌러야 합니다.")
	if ips := lanIPs(); len(ips) > 0 {
		fmt.Printf("  (자동으로 못 찾을 때: 참가자가 브라우저에서 http://%s:%s 로 직접 접속)\n", ips[0], uiPort)
	}
}
