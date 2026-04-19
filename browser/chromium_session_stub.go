//go:build !playwright

package browser

import (
	"errors"
	"os"
	"strings"
)

func openChromiumSession(m ChromiumMiddleware, opts BrowserSessionOptions) (*ChromiumSession, error) {
	spec := m.LaunchSpec(opts)
	if strings.TrimSpace(spec.URL) == "" {
		return nil, errors.New("browser middleware url is empty")
	}
	if strings.TrimSpace(spec.BrowserPath) == "" {
		return nil, errors.New("browser path is empty")
	}
	if _, err := os.Stat(spec.BrowserPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(spec.StealthScript.Path); err != nil {
		return nil, err
	}
	return nil, errors.New("playwright runtime is disabled in this build; use -tags playwright")
}

func closeChromiumSession(s *ChromiumSession) error {
	return nil
}

func chromiumSessionTitle(s *ChromiumSession) (string, error) {
	return "", errors.New("playwright runtime is disabled in this build; use -tags playwright")
}

func waitChromiumSessionClosed(s *ChromiumSession) error {
	return errors.New("playwright runtime is disabled in this build; use -tags playwright")
}

func chromiumSessionContent(s *ChromiumSession) (string, error) {
	return "", errors.New("playwright runtime is disabled in this build; use -tags playwright")
}

func chromiumSessionGoto(s *ChromiumSession, url string) error {
	return errors.New("playwright runtime is disabled in this build; use -tags playwright")
}

func chromiumSessionClickText(s *ChromiumSession, text string) error {
	return errors.New("playwright runtime is disabled in this build; use -tags playwright")
}
