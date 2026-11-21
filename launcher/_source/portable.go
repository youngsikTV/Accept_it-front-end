package main

// 포터블 모드: Docker·Java·Gradle 설치 없이, exe 옆의 runtime 폴더만으로 실행합니다.
//
//	AcceptIt.exe
//	items.txt
//	runtime/
//	  app.jar            ← gradlew bootJar 로 만든 백엔드
//	  jre/bin/java.exe   ← Java 21 (압축 버전)
//	  mysql/bin/mysqld.exe, mysql.exe, mysqladmin.exe   ← MySQL 8.0 (ZIP 버전)
//	data/                ← 실행 중 자동 생성 (DB 파일, 로그)
import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var dbPort = "3307" // 사용 중이면 빈 포트로 바뀝니다.

const (
	pDBName = "acceptit_db"
	pDBUser = "acceptit"
	pDBPass = "acceptit_local_pw" // DB 는 127.0.0.1 에서만 열려 있는 이 컴퓨터 전용입니다.
)

func exeDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	d, _ := os.Getwd()
	return d
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func bin(dir, name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

// portableBase: exe 옆(또는 현재 폴더)에 runtime/app.jar 가 있으면 그 폴더를 돌려줍니다.
func portableBase() string {
	wd, _ := os.Getwd()
	for _, d := range []string{exeDir(), wd} {
		if exists(filepath.Join(d, "runtime", "app.jar")) {
			return d
		}
	}
	return ""
}

func pMySQL(base, db, sql string) (string, error) {
	args := []string{"-h127.0.0.1", "-P" + dbPort, "-uroot", "--default-character-set=utf8mb4", "-N", "-B"}
	if db != "" {
		args = append(args, db)
	}
	c := exec.Command(bin(filepath.Join(base, "runtime", "mysql", "bin"), "mysql"), args...)
	c.Stdin = strings.NewReader(sql)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func tcpOpen(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func startPortableDB(base string) {
	rt := filepath.Join(base, "runtime")
	mysqld := bin(filepath.Join(rt, "mysql", "bin"), "mysqld")
	basedir := filepath.ToSlash(filepath.Join(rt, "mysql"))
	dataDir := filepath.Join(base, "data", "mysql")
	logPath := filepath.Join(base, "data", "mysqld.log")
	_ = os.MkdirAll(filepath.Join(base, "data"), 0o755)

	portFile := filepath.Join(base, "data", "db.port")
	reuse := false
	if b, err := os.ReadFile(portFile); err == nil { // 이전에 켠 이 게임의 DB 가 아직 떠 있으면 재사용
		if pp := strings.TrimSpace(string(b)); pp != "" && tcpOpen("127.0.0.1:"+pp) {
			dbPort = pp
			if out, _ := pMySQL(base, "", "SELECT 1"); isOK(out) {
				reuse = true
			}
		}
	}
	if reuse {
		fmt.Println("  이미 실행 중인 DB 를 사용합니다. (포트 " + dbPort + ")")
	} else {
		if !exists(filepath.Join(dataDir, "mysql")) {
			_ = os.RemoveAll(dataDir) // 이전에 중단된 초기화 찌꺼기 제거
			fmt.Println("  처음 실행이라 DB 를 준비합니다. (1분 정도 걸립니다)")
			initCmd := exec.Command(mysqld, "--initialize-insecure", "--console",
				"--basedir="+basedir, "--datadir="+filepath.ToSlash(dataDir))
			if out, err := initCmd.CombinedOutput(); err != nil {
				fail("DB 초기화 실패: %v\n%s\n(VC++ 재배포 패키지가 없으면 실패할 수 있습니다: https://aka.ms/vs/17/release/vc_redist.x64.exe)", err, tailString(string(out), 15))
			}
		}
		dbPort = freePort(3307)
		if dbPort != "3307" {
			fmt.Println("  3307 번 포트가 사용 중이라 DB 는 " + dbPort + " 번 포트를 씁니다.")
		}
		logf, _ := os.Create(logPath)
		db := exec.Command(mysqld, "--console", "--basedir="+basedir, "--datadir="+filepath.ToSlash(dataDir),
			"--port="+dbPort, "--bind-address=127.0.0.1", "--mysqlx=OFF", "--character-set-server=utf8mb4")
		db.Stdout, db.Stderr = logf, logf
		if err := db.Start(); err != nil {
			fail("MySQL 실행 실패: %v", err)
		}
		_ = os.WriteFile(portFile, []byte(dbPort), 0o644)
		exited := make(chan struct{})
		go func() { _ = db.Wait(); close(exited) }()
		onExit(func() {
			fmt.Println("  DB 를 안전하게 종료하는 중...")
			_ = exec.Command(bin(filepath.Join(rt, "mysql", "bin"), "mysqladmin"), "-h127.0.0.1", "-P"+dbPort, "-uroot", "shutdown").Run()
			select {
			case <-exited:
			case <-time.After(30 * time.Second):
				killTree(db)
			}
		})
		end := time.Now().Add(3 * time.Minute)
		for {
			if out, _ := pMySQL(base, "", "SELECT 1"); isOK(out) {
				break
			}
			select {
			case <-exited:
				cleanupAll()
				fail("MySQL 이 시작 중 종료되었습니다. 마지막 로그:\n%s\n(로그: %s)", tailFile(logPath, 20), logPath)
			default:
			}
			if time.Now().After(end) {
				cleanupAll()
				fail("MySQL 이 3분 안에 준비되지 않았습니다. 로그: %s", logPath)
			}
			time.Sleep(2 * time.Second)
			fmt.Print(".")
		}
	}
	setup := "CREATE DATABASE IF NOT EXISTS `" + pDBName + "`;\n" +
		"CREATE USER IF NOT EXISTS '" + pDBUser + "'@'%' IDENTIFIED WITH mysql_native_password BY '" + pDBPass + "';\n" +
		"GRANT ALL PRIVILEGES ON `" + pDBName + "`.* TO '" + pDBUser + "'@'%';\nFLUSH PRIVILEGES;\n"
	if out, err := pMySQL(base, "", setup); err != nil {
		cleanupAll()
		fail("DB 계정 준비 실패: %v\n%s", err, out)
	}
	fmt.Println("\n  MySQL 준비 완료")
}

func tailString(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func runPortable(base string, syncOnly, noOpen bool) {
	fmt.Println("  [설치 없이 실행 모드] 폴더:", base)
	base = asciiBase(base)
	rt := filepath.Join(base, "runtime")
	var missing []string
	for _, p := range []string{
		filepath.Join(rt, "app.jar"),
		bin(filepath.Join(rt, "jre", "bin"), "java"),
		bin(filepath.Join(rt, "mysql", "bin"), "mysqld"),
		bin(filepath.Join(rt, "mysql", "bin"), "mysql"),
		bin(filepath.Join(rt, "mysql", "bin"), "mysqladmin"),
	} {
		if !exists(p) {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		fail("runtime 폴더에 필요한 파일이 없습니다:\n  %s\n(런처 사용법.txt 의 '파일 준비' 단계를 확인하세요)", strings.Join(missing, "\n  "))
	}

	step("1/4 물품 목록(items.txt) 확인")
	items := loadItems(base)

	if syncOnly {
		if b, err := os.ReadFile(filepath.Join(base, "data", "db.port")); err == nil {
			dbPort = strings.TrimSpace(string(b))
		}
		if out, err := pMySQL(base, pDBName, buildSyncSQL(items)); err != nil {
			fail("물품 반영 실패 (게임이 켜져 있어야 합니다): %v\n%s", err, out)
		}
		fmt.Println("  물품 목록을 DB에 반영했습니다.")
		applyKeywords(func(sql string) (string, error) { return pMySQL(base, pDBName, sql) })
		cleanupAll()
		return
	}

	installExitHandlers()
	bp := freePort(8080)
	if bp != "8080" {
		fmt.Println("  8080 번 포트가 사용 중이라 백엔드는 " + bp + " 번 포트를 씁니다.")
	}
	backendURL = "http://127.0.0.1:" + bp
	startUI(noOpen) // 화면을 먼저 열어 둡니다. 참가만 할 사람은 서버 준비를 기다리지 않아도 됩니다.
	step("2/4 데이터베이스(MySQL) 시작")
	startPortableDB(base)

	step("3/4 백엔드 서버 시작 (처음에는 1~2분 걸릴 수 있습니다)")
	logPath := filepath.Join(base, "data", "acceptit-backend.log")
	be := exec.Command(bin(filepath.Join(rt, "jre", "bin"), "java"), "-Dfile.encoding=UTF-8", "-jar", filepath.Join(rt, "app.jar"))
	be.Dir = rt // runtime 폴더에는 .env 가 없으므로 아래 환경변수가 쓰입니다.
	be.Env = append(os.Environ(), "DB_NAME="+pDBName, "DB_USER="+pDBUser, "DB_PASSWORD="+pDBPass, "SWAGGER_ENABLED=false",
		"SERVER_PORT="+bp, "SPRING_DATASOURCE_URL=jdbc:mysql://127.0.0.1:"+dbPort+"/"+pDBName)
	logf, _ := os.Create(logPath)
	be.Stdout, be.Stderr = logf, logf
	if err := be.Start(); err != nil {
		cleanupAll()
		fail("백엔드 시작 실패: %v", err)
	}
	onExit(func() { killTree(be) })
	waitBackend(be, logPath)

	step("4/4 물품 목록을 DB에 반영하고 게임 화면 열기")
	if out, err := pMySQL(base, pDBName, buildSyncSQL(items)); err != nil {
		cleanupAll()
		fail("물품 반영 실패: %v\n%s", err, out)
	}
	fmt.Printf("  물품 %d개 반영 완료. 게임 시작 때 이 목록에서 무작위로 출제됩니다.\n", len(items))
	applyKeywords(func(sql string) (string, error) { return pMySQL(base, pDBName, sql) })
	localReady.Store(true)
	fmt.Println("\n★ 이 컴퓨터의 서버 준비 완료 — 이제 Host 로 방을 만들 수 있습니다.")
	select {}
}

func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// asciiBase: 폴더 경로에 한글 등이 있으면 MySQL 이 실행되지 않을 수 있으므로,
// 그 폴더를 비어 있는 드라이브 글자(Z:, Y:, ...)에 연결(subst)해 영문 경로로 다루게 합니다.
// 파일은 원래 위치에 그대로 있으며, 종료할 때 연결을 해제합니다. 실패하면 원래 경로를 그대로 씁니다.
func asciiBase(base string) string {
	if runtime.GOOS != "windows" || !hasNonASCII(base) {
		return base
	}
	for l := 'Z'; l >= 'G'; l-- {
		drive := string(l) + ":"
		if exists(drive + `\`) {
			continue
		}
		if err := exec.Command("subst", drive, base).Run(); err != nil {
			continue
		}
		if !exists(filepath.Join(drive+`\`, "runtime", "app.jar")) {
			_ = exec.Command("subst", drive, "/d").Run()
			continue
		}
		onExit(func() { _ = exec.Command("subst", drive, "/d").Run() })
		fmt.Printf("  폴더 경로에 한글이 있어 %s 드라이브로 연결해 실행합니다. (파일은 원래 위치에 그대로 있습니다)\n", drive)
		return drive + `\`
	}
	fmt.Println("  [주의] 폴더 경로에 한글 등이 있고 가상 드라이브 연결에 실패했습니다. DB 가 안 켜지면 C:\\AcceptIt 처럼 영문 경로로 옮기세요.")
	return base
}
