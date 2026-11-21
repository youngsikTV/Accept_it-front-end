// AcceptIt 실행기: MySQL(Docker) → 백엔드(Gradle bootRun) → 물품 목록 반영 → 게임 화면 제공
// 기존 backend(src/)·frontend(frontend/) 소스는 읽기만 하고 수정하지 않습니다.
package main

import (
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
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed all:web
var webFS embed.FS

const (
	containerDB = "AcceptIt_db" // docker-compose.yml 의 container_name
)

// 포트는 기본값에서 시작하며, 설치 없는 모드에서는 사용 중이면 빈 포트로 자동 변경됩니다.
var (
	uiPort     = "5173"
	backendURL = "http://127.0.0.1:8080"
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
	s := strings.ToLower(readLine("\n" + q + " (Y/N): "))
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
	if tcpOpen("127.0.0.1:" + port) { // 127.0.0.1 에만 열린 프로그램도 감지
		return true
	}
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true
	}
	l.Close()
	return false
}

// freePort: start 부터 위로 올라가며 비어 있는 포트를 찾습니다.
func freePort(start int) string {
	for p := start; p < start+300; p++ {
		if s := strconv.Itoa(p); !portBusy(s) {
			return s
		}
	}
	fail("%d 번 이후로 비어 있는 포트를 찾지 못했습니다.", start)
	return ""
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

// dockerMySQL: Docker 의 MySQL 컨테이너에 SQL 을 실행합니다.
func dockerMySQL(env map[string]string, sql string) (string, error) {
	c := exec.Command("docker", "exec", "-i", containerDB, "mysql", "-uroot", "-p"+env["DB_ROOT_PASSWORD"],
		"--default-character-set=utf8mb4", env["DB_NAME"])
	c.Stdin = strings.NewReader(sql)
	out, err := c.CombinedOutput()
	return string(out), err
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
	loadKeywords(root)
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

var exitOnce sync.Once

func installExitHandlers() { exitOnce.Do(installExitHandlersOnce) }

func installExitHandlersOnce() {
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

// newLocalProxy: 이 컴퓨터의 백엔드로 보내는 프록시.
// 백엔드의 WebSocket 설정 중 한 곳이 Origin 을 http://localhost:5173 으로만 허용합니다.
// 포트가 바뀌거나 다른 컴퓨터에서 접속해도 연결되도록 /ws 요청의 Origin 을 맞춰 줍니다. (백엔드 소스는 그대로)
func newLocalProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = snoopRoomCode
	dir := proxy.Director
	proxy.Director = func(r *http.Request) {
		dir(r)
		if strings.HasPrefix(r.URL.Path, "/ws/") {
			r.Header.Set("Origin", "http://localhost:5173")
		}
	}
	return proxy
}

// startUI: 내장된 게임 화면을 :5173(사용 중이면 다른 포트)에 제공합니다. (바로 돌아옴)
//   - /api, /ws 는 이 컴퓨터의 백엔드로 보내되, 다른 컴퓨터의 방 코드면 그 방장 컴퓨터로 대신 전달합니다. (route.go)
//   - 같은 네트워크의 다른 컴퓨터가 방장을 찾을 때 응답합니다. (discovery.go)
func startUI(noOpen bool) {
	if portBusy(uiPort) {
		old := uiPort
		n, _ := strconv.Atoi(uiPort)
		uiPort = freePort(n + 1)
		fmt.Printf("  화면 포트 %s 는 사용 중이라 %s 번으로 엽니다.\n", old, uiPort)
	}
	sub, _ := fs.Sub(webFS, "web")
	target, _ := url.Parse(backendURL)
	proxy := newLocalProxy(target)
	api := apiHandler(proxy)
	go serveDiscovery(discPort, uiPort)
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/ws/", api)
	files := http.FileServer(http.FS(sub))
	idx, _ := fs.ReadFile(sub, "index.html")
	indexHTML := injectBanner(idx) // 방장 컴퓨터 메인 화면에 '같이 하기 주소' 상자를 끼워 넣음
	mux.HandleFunc("/__acceptit/info", infoHandler)
	mux.HandleFunc("/__acceptit/banner.js", bannerHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if f, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/")); err == nil {
				f.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		// /room /game /result 새로고침 대응
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(indexHTML)
	})
	ln, err := net.Listen("tcp", ":"+uiPort)
	if err != nil {
		fail("화면 서버 시작 실패 (포트 %s 사용 중?): %v", uiPort, err)
	}
	go func() { _ = http.Serve(ln, mux) }()
	addr := "http://localhost:" + uiPort
	fmt.Printf("\n게임 주소: %s\n끝내려면 이 창에서 Ctrl+C 또는 창 닫기\n", addr)
	printShareInfo()
	if !noOpen {
		openBrowser(addr)
	}
	if publicMode {
		installExitHandlers()
		go startTunnel(uiPort)
	}
}

// serveUI: 화면을 열고 이 컴퓨터의 서버가 준비되었음을 알린 뒤 계속 실행합니다. (Docker 모드용)
func serveUI(noOpen bool) {
	startUI(noOpen)
	localReady.Store(true)
	select {}
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
		applyKeywords(func(sql string) (string, error) { return dockerMySQL(env, sql) })
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
	applyKeywords(func(sql string) (string, error) { return dockerMySQL(env, sql) })
	serveUI(noOpen)
}

func main() {
	syncOnly := flag.Bool("sync-only", false, "물품 목록만 DB에 반영하고 종료 (서버가 이미 켜져 있을 때)")
	noOpen := flag.Bool("no-browser", false, "브라우저를 자동으로 열지 않음")
	forceDocker := flag.Bool("docker", false, "runtime 폴더가 있어도 Docker 모드로 실행")
	menu := flag.Bool("menu", false, "시작할 때 방장/참가자 선택 메뉴를 보여 줌 (보통은 필요 없음)")
	public := flag.Bool("public", false, "인터넷 공개: 와이파이가 달라도 접속할 수 있는 주소를 만듭니다 (cloudflared 필요)")
	join := flag.String("join", "", "참가자로 접속할 방장 컴퓨터 주소 (예: 192.168.0.12:5173)")
	flag.Parse()

	fmt.Println("=== ACCEPT IT 실행기 ===")
	publicMode = *public
	role := "host" // 기본: 모두 같은 방식으로 실행. 화면의 Host/Join 과 방 코드로 자동 연결됩니다.
	switch {
	case *join != "":
		role = "join"
	case *menu && !*syncOnly:
		role = chooseRole()
	}
	switch role {
	case "join":
		runJoin(*join, *noOpen)
		return
	case "join-auto":
		runJoinAuto(*noOpen)
		return
	}
	if !*forceDocker {
		if base := portableBase(); base != "" {
			runPortable(base, *syncOnly, *noOpen)
			return
		}
	}
	runDocker(*syncOnly, *noOpen)
}
