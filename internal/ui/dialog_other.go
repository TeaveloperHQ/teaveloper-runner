//go:build !windows

package ui

import (
	"fmt"
	"os"
)

// FatalError 는 콘솔에 오류를 출력한다(개발용).
func FatalError(title, body string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", title, body)
}

// Info 는 콘솔에 안내를 출력한다(개발용).
func Info(title, body string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", title, body)
}
