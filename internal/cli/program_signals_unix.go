//go:build darwin || linux || freebsd || openbsd || netbsd

package cli

import (
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

func runProgram(prog *tea.Program) (tea.Model, error) {
	sigCh := make(chan os.Signal, 1)
	done := make(chan struct{})
	// Bubble Tea handles SIGINT/SIGTERM, but SIGQUIT keeps Go's default
	// goroutine dump unless we claim it explicitly. Recording tools can use
	// SIGQUIT/SIGHUP to stop a capture, so route both through normal teardown.
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGQUIT)
	defer func() {
		signal.Stop(sigCh)
		close(done)
	}()

	go func() {
		select {
		case <-sigCh:
			prog.Quit()
		case <-done:
		}
	}()

	return prog.Run()
}
