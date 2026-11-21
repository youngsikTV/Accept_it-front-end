//go:build windows

package main

import "syscall"

// 콘솔 창의 X 버튼/로그오프/종료 신호를 받으면 정리 작업(DB 정상 종료 등)을 실행합니다.
func installConsoleCloseHandler(f func()) {
	cb := syscall.NewCallback(func(ev uint32) uintptr {
		f()
		return 1
	})
	syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler").Call(cb, 1)
}
