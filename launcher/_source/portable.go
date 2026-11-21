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
	args := []string{"-h127.0.0.1", "-P3307", "-uroot", "--default-character-set=utf8mb4", "-N", "-B"}
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

	if tcpOpen("127.0.0.1:3307") {
		out, _ := pMySQL(base, "", "SELECT 1")
		if !isOK(out) {
			fail("포트 3307 을 다른 프로그램이 쓰고 있습니다. (예: Docker 의 MySQL)\n그 프로그램을 끄고 다시 실행하세요. docker 컨테이너라면: docker stop 컨테이너이름")
		}
		fmt.Println("  이미 실행 중인 DB 를 사용합니다.")
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
		logf, _ := os.Create(logPath)
		db := exec.Command(mysqld, "--console", "--basedir="+basedir, "--datadir="+filepath.ToSlash(dataDir),
			"--port=3307", "--bind-address=127.0.0.1", "--mysqlx=OFF", "--character-set-server=utf8mb4")
		db.Stdout, db.Stderr = logf, logf
		if err := db.Start(); err != nil {
			fail("MySQL 실행 실패: %v", err)
		}
		exited := make(chan struct{})
		go func() { _ = db.Wait(); close(exited) }()
		onExit(func() {
			fmt.Println("  DB 를 안전하게 종료하는 중...")
			_ = exec.Command(bin(filepath.Join(rt, "mysql", "bin"), "mysqladmin"), "-h127.0.0.1", "-P3307", "-uroot", "shutdown").Run()
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
	rt := filepath.Join(base, "runtime")
	fmt.Println("  [설치 없이 실행 모드] 폴더:", base)
	for _, r := range base {
		if r > 127 {
			fmt.Println("  [주의] 폴더 경로에 한글 등이 있습니다. DB 실행이 안 되면 C:\\AcceptIt 처럼 영문 경로로 옮기세요.")
			break
		}
	}
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
		if out, err := pMySQL(base, pDBName, buildSyncSQL(items)); err != nil {
			fail("물품 반영 실패 (게임이 켜져 있어야 합니다): %v\n%s", err, out)
		}
		fmt.Println("  물품 목록을 DB에 반영했습니다.")
		return
	}

	installExitHandlers()
	step("2/4 데이터베이스(MySQL) 시작")
	startPortableDB(base)

	step("3/4 백엔드 서버 시작 (처음에는 1~2분 걸릴 수 있습니다)")
	logPath := filepath.Join(base, "data", "acceptit-backend.log")
	if healthy() {
		fmt.Println("  이미 실행 중인 백엔드를 사용합니다.")
	} else {
		if portBusy("8080") {
			cleanupAll()
			fail("포트 8080 을 다른 프로그램이 쓰고 있습니다. 그 프로그램을 끄고 다시 실행하세요.")
		}
		be := exec.Command(bin(filepath.Join(rt, "jre", "bin"), "java"), "-Dfile.encoding=UTF-8", "-jar", filepath.Join(rt, "app.jar"))
		be.Dir = rt // runtime 폴더에는 .env 가 없으므로 아래 환경변수가 쓰입니다.
		be.Env = append(os.Environ(), "DB_NAME="+pDBName, "DB_USER="+pDBUser, "DB_PASSWORD="+pDBPass, "SWAGGER_ENABLED=false")
		logf, _ := os.Create(logPath)
		be.Stdout, be.Stderr = logf, logf
		if err := be.Start(); err != nil {
			cleanupAll()
			fail("백엔드 시작 실패: %v", err)
		}
		onExit(func() { killTree(be) })
		waitBackend(be, logPath)
	}

	step("4/4 물품 목록을 DB에 반영하고 게임 화면 열기")
	if out, err := pMySQL(base, pDBName, buildSyncSQL(items)); err != nil {
		cleanupAll()
		fail("물품 반영 실패: %v\n%s", err, out)
	}
	fmt.Printf("  물품 %d개 반영 완료. 게임 시작 때 이 목록에서 무작위로 출제됩니다.\n", len(items))
	serveUI(noOpen)
}
