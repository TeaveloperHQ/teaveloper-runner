//go:build !windows

package ui

import "os/exec"

// openURL 은 기본 브라우저로 url 을 연다(개발용; Linux/macOS).
func openURL(url string) error {
	for _, c := range [][]string{{"xdg-open", url}, {"open", url}} {
		if _, err := exec.LookPath(c[0]); err == nil {
			return exec.Command(c[0], c[1:]...).Start()
		}
	}
	return nil
}
