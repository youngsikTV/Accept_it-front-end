// AcceptIt 실행기: MySQL(Docker) → 백엔드(Gradle bootRun) → 물품 목록 반영 → 게임 화면 제공
// 기존 backend(src/)·frontend(frontend/) 소스는 읽기만 하고 수정하지 않습니다.
package main

import (
	"bufio"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed all:web
var webFS embed.FS

const (
	uiPort      = "5173"
	backendURL  = "http://127.0.0.1:8080"
	containerDB = "AcceptIt_db" // docker-compose.yml 의 container_name
)

func fail(format string, a ...any) {
	cleanupAll() // 오류로 끝날 때도 켜 둔 DB/백엔드를 정리
	fmt.Printf("\n[오류] "+format+"\n", a...)
	if runtime.GOOS == "windows" {
		fmt.Print("\n아무 키나 누르면 닫힙니다...")
		fmt.Scanln()
	}
	os.Exit(1)
}

func step(format string, a ...any) { fmt.Printf("\n▶ "+format+"\n", a...) }

func findRoot() string {
	var starts []string
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	for _, d := range starts {
		for i := 0; i < 5; i++ {
			if _, err := os.Stat(filepath.Join(d, "docker-compose.yml")); err == nil {
				if _, err := os.Stat(filepath.Join(d, "build.gradle")); err == nil {
					return d
				}
			}
			d = filepath.Dir(d)
		}
	}
	return ""
}

func readEnv(path string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ensureEnv: .env 가 없으면 이 컴퓨터 전용 DB 계정으로 새로 만듭니다. (이미 있으면 절대 덮어쓰지 않음)
func ensureEnv(root string) map[string]string {
	p := filepath.Join(root, ".env")
	if _, err := os.Stat(p); err != nil {
		content := "DB_NAME=acceptit_db\nDB_USER=acceptit\nDB_PASSWORD=" + randHex(8) +
			"\nDB_ROOT_PASSWORD=" + randHex(8) + "\nOPENAI_API_KEY=\nOPENAI_MODEL=gpt-4o-mini\n"
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			fail(".env 를 만들지 못했습니다: %v", err)
		}
		fmt.Println("  .env 가 없어 새로 만들었습니다. (git 에는 올라가지 않는 파일입니다)")
	}
	env := readEnv(p)
	for _, k := range []string{"DB_NAME", "DB_USER", "DB_PASSWORD", "DB_ROOT_PASSWORD"} {
		if env[k] == "" {
			fail(".env 에 %s 값이 비어 있습니다.", k)
		}
	}
	return env
}

func run(root string, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = root
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func waitFor(what string, timeout time.Duration, ok func() bool) {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if ok() {
			return
		}
		time.Sleep(2 * time.Second)
		fmt.Print(".")
	}
	fail("%s 이(가) %v 안에 준비되지 않았습니다.", what, timeout)
}

func dockerOut(args ...string) string {
	out, _ := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out))
}

func ask(q string) bool {
	fmt.Printf("\n%s (Y/N): ", q)
	s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes" || s == "ㅛ"
}

func tailFile(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func healthy() bool {
	c := http.Client{Timeout: 3 * time.Second}
	r, err := c.Get(backendURL + "/actuator/health")
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == 200
}

func portBusy(port string) bool {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true
	}
	l.Close()
	return false
}

// 다른 프로젝트의 컨테이너가 3307/8080 을 쓰고 있으면 (사용자 동의 후) 잠시 멈춥니다. 삭제는 하지 않습니다.
func freeConflicts() {
	out := dockerOut("ps", "--format", "{{.Names}}\t{{.Ports}}")
	for _, l := range strings.Split(out, "\n") {
		name, ports, ok := strings.Cut(strings.TrimSpace(l), "\t")
		if !ok || name == containerDB {
			continue
		}
		if strings.Contains(ports, ":3307->") || strings.Contains(ports, ":8080->") {
			if ask("다른 프로젝트 컨테이너 '" + name + "' 가 포트(3307/8080)를 쓰고 있습니다. 잠시 멈출까요? (삭제되지 않고 `docker start " + name + "` 로 다시 켤 수 있습니다)") {
				dockerOut("stop", name)
			} else {
				fail("포트 충돌: '%s' 를 멈추지 않으면 ACCEPT IT 를 실행할 수 없습니다.", name)
			}
		}
	}
}

func isOK(out string) bool {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1]) == "1"
}

func mysqlLogin(user, pw string) string {
	return dockerOut("exec", containerDB, "mysql", "-h127.0.0.1", "-u"+user, "-p"+pw, "-N", "-B", "-e", "SELECT 1")
}

// startDB: MySQL 컨테이너를 (포트 연결까지 확인하며) 켜고 접속 가능해질 때까지 기다립니다.
func startDB(root string, env map[string]string) {
	freeConflicts()
	if err := run(root, "docker", "compose", "up", "-d", "mysql-db"); err != nil {
		fail("MySQL 시작 실패 (Docker Desktop 이 켜져 있나요?): %v", err)
	}
	if !strings.Contains(dockerOut("port", containerDB, "3306/tcp"), ":3307") {
		fmt.Println("  DB 포트 연결이 빠져 있어 컨테이너를 다시 만듭니다.")
		if err := run(root, "docker", "compose", "up", "-d", "--force-recreate", "mysql-db"); err != nil {
			fail("MySQL 재생성 실패: %v", err)
		}
	}
	reset := false
	for {
		var out string
		waitFor("MySQL", 3*time.Minute, func() bool {
			out = mysqlLogin("root", env["DB_ROOT_PASSWORD"])
			return isOK(out) || strings.Contains(out, "Access denied")
		})
		bad := strings.Contains(out, "Access denied") ||
			strings.Contains(mysqlLogin(env["DB_USER"], env["DB_PASSWORD"]), "Access denied")
		if !bad {
			break
		}
		if reset {
			fail("DB 계정이 .env 와 계속 맞지 않습니다. .env 의 DB_* 값을 확인하세요.")
		}
		if !ask("DB 에 저장된 계정 정보가 .env 와 달라 접속할 수 없습니다. DB 를 초기화할까요? (이 프로젝트의 테스트 데이터만 지워지며, 물품은 items.txt 로 다시 채워집니다)") {
			fail("DB 계정 불일치로 진행할 수 없습니다.")
		}
		_ = run(root, "docker", "compose", "down", "-v")
		if err := run(root, "docker", "compose", "up", "-d", "mysql-db"); err != nil {
			fail("MySQL 시작 실패: %v", err)
		}
		reset = true
	}
	waitFor("DB 포트(3307)", 60*time.Second, func() bool {
		c, err := net.DialTimeout("tcp", "127.0.0.1:3307", 2*time.Second)
		if err != nil {
			return false
		}
		c.Close()
		return true
	})
}

func syncItems(root string, env map[string]string, items []Item) error {
	c := exec.Command("docker", "exec", "-i", containerDB, "mysql", "-uroot", "-p"+env["DB_ROOT_PASSWORD"],
		"--default-character-set=utf8mb4", env["DB_NAME"])
	c.Dir = root
	c.Stdin = strings.NewReader(buildSyncSQL(items))
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, out)
	}
	return nil
}

func loadItems(root string) []Item {
	var path string
	exeDir := "."
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	for _, d := range []string{exeDir, filepath.Join(root, "launcher"), root} {
		if _, err := os.Stat(filepath.Join(d, "items.txt")); err == nil {
			path = filepath.Join(d, "items.txt")
			break
		}
	}
	if path == "" {
		fail("items.txt 를 찾지 못했습니다. AcceptIt.exe 와 같은 폴더에 두세요.")
	}
	b, _ := os.ReadFile(path)
	items, err := parseItems(string(b))
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("  %s 에서 물품 %d개를 읽었습니다.\n", path, len(items))
	return items
}

func killTree(c *exec.Cmd) {
	if c == nil || c.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(c.Process.Pid)).Run()
	} else {
		_ = c.Process.Kill()
	}
}

var (
	cleanups   []func()
	cleanupMu  sync.Mutex
	cleanupRan bool
)

func onExit(f func()) {
	cleanupMu.Lock()
	cleanups = append([]func(){f}, cleanups...) // 나중에 켠 것부터 끈다
	cleanupMu.Unlock()
}

func cleanupAll() {
	cleanupMu.Lock()
	if cleanupRan {
		cleanupMu.Unlock()
		return
	}
	cleanupRan = true
	list := cleanups
	cleanupMu.Unlock()
	for _, f := range list {
		f()
	}
}

func installExitHandlers() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() { <-stop; fmt.Println("\n종료하는 중..."); cleanupAll(); os.Exit(0) }()
	installConsoleCloseHandler(func() { cleanupAll(); os.Exit(0) }) // 창 X 버튼 (Windows)
}

// waitBackend: /actuator/health 가 200 이 될 때까지 기다리고, 도중에 죽으면 로그를 보여 줍니다.
func waitBackend(be *exec.Cmd, logPath string) {
	exited := make(chan struct{})
	go func() { _ = be.Wait(); close(exited) }()
	end := time.Now().Add(10 * time.Minute)
	for !healthy() {
		select {
		case <-exited:
			cleanupAll()
			fail("백엔드가 시작 중 종료되었습니다. 마지막 로그:\n%s\n\n(전체 로그: %s)", tailFile(logPath, 25), logPath)
		default:
		}
		if time.Now().After(end) {
			cleanupAll()
			fail("백엔드가 10분 안에 준비되지 않았습니다. 로그: %s", logPath)
		}
		time.Sleep(2 * time.Second)
		fmt.Print(".")
	}
	fmt.Println("\n  백엔드 준비 완료")
}

// serveUI: 내장된 게임 화면을 :5173 에 제공하고 /api, /ws 는 백엔드(:8080)로 넘깁니다.
func serveUI(noOpen bool) {
	sub, _ := fs.Sub(webFS, "web")
	target, _ := url.Parse(backendURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	mux := http.NewServeMux()
	mux.Handle("/api/", proxy)
	mux.Handle("/ws/", proxy)
	files := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if f, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/")); err == nil {
				f.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/" // /room /game /result 새로고침 대응
		files.ServeHTTP(w, r)
	})
	addr := "http://localhost:" + uiPort
	fmt.Printf("\n게임 주소: %s\n(같은 와이파이의 다른 사람은 이 컴퓨터 IP:%s 로 접속)\n끝내려면 이 창에서 Ctrl+C 또는 창 닫기\n", addr, uiPort)
	if !noOpen {
		switch runtime.GOOS {
		case "windows":
			_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", addr).Start()
		case "darwin":
			_ = exec.Command("open", addr).Start()
		}
	}
	if err := http.ListenAndServe(":"+uiPort, mux); err != nil {
		cleanupAll()
		fail("화면 서버 시작 실패 (포트 %s 사용 중?): %v", uiPort, err)
	}
}

func runDocker(syncOnly, noOpen bool) {
	root := findRoot()
	if root == "" {
		fail("프로젝트 폴더(docker-compose.yml, build.gradle 이 있는 곳)를 찾지 못했습니다.\nAcceptIt.exe 를 프로젝트 폴더 안에 두고 실행하세요.\n(Docker 없이 쓰려면 runtime 폴더(app.jar, jre, mysql)를 exe 옆에 두세요.)")
	}
	fmt.Println("  [Docker 모드] 프로젝트 폴더:", root)

	step("1/5 물품 목록(items.txt) 확인")
	items := loadItems(root)
	step("2/5 설정(.env) 확인")
	env := ensureEnv(root)

	if syncOnly {
		if err := syncItems(root, env, items); err != nil {
			fail("물품 반영 실패: %v", err)
		}
		fmt.Println("  물품 목록을 DB에 반영했습니다.")
		return
	}

	if _, err := exec.LookPath("docker"); err != nil {
		fail("Docker 가 설치되어 있지 않거나 실행 중이 아닙니다. Docker Desktop 을 켜고 다시 실행하세요.")
	}
	step("3/5 데이터베이스(MySQL) 시작")
	startDB(root, env)
	fmt.Println("\n  MySQL 준비 완료")

	step("4/5 백엔드 서버 시작 (처음에는 몇 분 걸릴 수 있습니다)")
	logPath := filepath.Join(root, "acceptit-backend.log")
	if healthy() {
		fmt.Println("  이미 실행 중인 백엔드를 사용합니다.")
	} else {
		if portBusy("8080") {
			fail("포트 8080 을 다른 프로그램이 쓰고 있습니다. 그 프로그램을 끄고 다시 실행하세요.")
		}
		var be *exec.Cmd
		if runtime.GOOS == "windows" {
			be = exec.Command("cmd", "/C", filepath.Join(root, "gradlew.bat"), "bootRun")
		} else {
			be = exec.Command(filepath.Join(root, "gradlew"), "bootRun")
		}
		be.Dir = root
		logf, _ := os.Create(logPath)
		be.Stdout, be.Stderr = logf, logf
		if err := be.Start(); err != nil {
			fail("백엔드 시작 실패 (Java 21 설치 필요): %v", err)
		}
		onExit(func() { killTree(be) })
		installExitHandlers()
		waitBackend(be, logPath)
	}

	step("5/5 물품 목록을 DB에 반영하고 게임 화면 열기")
	if err := syncItems(root, env, items); err != nil {
		cleanupAll()
		fail("물품 반영 실패: %v", err)
	}
	fmt.Printf("  물품 %d개 반영 완료. 게임 시작 때 이 목록에서 무작위로 출제됩니다.\n", len(items))
	serveUI(noOpen)
}

func main() {
	syncOnly := flag.Bool("sync-only", false, "물품 목록만 DB에 반영하고 종료 (서버가 이미 켜져 있을 때)")
	noOpen := flag.Bool("no-browser", false, "브라우저를 자동으로 열지 않음")
	forceDocker := flag.Bool("docker", false, "runtime 폴더가 있어도 Docker 모드로 실행")
	flag.Parse()

	fmt.Println("=== ACCEPT IT 실행기 ===")
	if !*forceDocker {
		if base := portableBase(); base != "" {
			runPortable(base, *syncOnly, *noOpen)
			return
		}
	}
	runDocker(*syncOnly, *noOpen)
}
