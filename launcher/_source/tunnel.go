package main

// 인터넷 공개(-public): 방장 컴퓨터의 게임 화면을 Cloudflare 임시 터널로 열어,
// 와이파이/네트워크가 달라도 주소(https://....trycloudflare.com) 하나로 접속하게 합니다.
// 참가자는 실행기 없이 브라우저만으로 Join 할 수 있습니다. (cloudflared 가 필요: runtime\cloudflared.exe 또는 PATH)
import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

var publicMode bool

var tunnelRe = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

func parseTunnelURL(line string) string { return tunnelRe.FindString(line) }

func findCloudflared() string {
	for _, p := range []string{
		bin(filepath.Join(exeDir(), "runtime"), "cloudflared"),
		bin(exeDir(), "cloudflared"),
	} {
		if exists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	return ""
}

func startTunnel(port string) {
	setTunnel("starting", "")
	cf := findCloudflared()
	if cf == "" {
		setTunnel("missing", "")
		fmt.Println("\n[인터넷 공개] cloudflared 를 찾지 못했습니다.")
		fmt.Println("  runtime 폴더에 cloudflared.exe 를 넣거나 make-bundle.bat 으로 다시 만드세요.")
		fmt.Println("  (https://github.com/cloudflare/cloudflared/releases/latest 의 cloudflared-windows-amd64.exe)")
		return
	}
	c := exec.Command(cf, "tunnel", "--no-autoupdate", "--url", "http://localhost:"+port)
	out, err := c.StderrPipe()
	if err != nil {
		setTunnel("failed", "")
		return
	}
	if err := c.Start(); err != nil {
		setTunnel("failed", "")
		fmt.Println("\n[인터넷 공개] cloudflared 실행 실패:", err)
		return
	}
	onExit(func() { killTree(c) })
	fmt.Println("\n[인터넷 공개] 주소를 만드는 중... (10~30초)")
	found := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if u := parseTunnelURL(sc.Text()); u != "" {
				select {
				case found <- u:
				default:
				}
			}
		}
	}()
	select {
	case u := <-found:
		setTunnel("ready", u)
		fmt.Println("\n==================================================================")
		fmt.Println("  인터넷 주소:", u)
		fmt.Println("  이 주소를 참가자에게 알려 주세요. 참가자는 AcceptIt.exe 없이")
		fmt.Println("  브라우저(휴대폰도 가능)로 열고 Join 에 방 코드를 입력하면 됩니다.")
		fmt.Println("  ※ 주소를 아는 누구나 접속할 수 있으니 게임이 끝나면 이 창을 닫으세요.")
		fmt.Println("  ※ 실행할 때마다 주소가 바뀌고, 방장 컴퓨터가 켜져 있어야 합니다.")
		fmt.Println("==================================================================")
	case <-time.After(60 * time.Second):
		setTunnel("failed", "")
		fmt.Println("\n[인터넷 공개] 60초 안에 주소를 받지 못했습니다. 인터넷 연결을 확인하거나 다시 실행하세요.")
	}
}
