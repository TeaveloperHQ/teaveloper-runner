//go:build !windows

package main

// 개발(콘솔) 환경에서는 항상 콘솔 출력을 함께 사용한다.
func isConsole() bool { return true }
