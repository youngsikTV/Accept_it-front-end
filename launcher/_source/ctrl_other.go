//go:build !windows

package main

func installConsoleCloseHandler(f func()) {}
