package main

import (
	"net"
	"strconv"
	"testing"
)

func TestFreePortSkipsBusy(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0") // 127.0.0.1 에만 열린 포트도 '사용 중'으로 감지해야 함
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	busy := l.Addr().(*net.TCPAddr).Port
	if !portBusy(strconv.Itoa(busy)) {
		t.Fatalf("사용 중인 포트 %d 를 감지하지 못함", busy)
	}
	got, _ := strconv.Atoi(freePort(busy))
	if got == busy || got < busy {
		t.Fatalf("freePort 가 사용 중인 포트를 돌려줌: %d", got)
	}
}
