//go:build !(darwin || linux || freebsd || openbsd || netbsd)

package cli

import tea "github.com/charmbracelet/bubbletea"

func runProgram(prog *tea.Program) (tea.Model, error) {
	return prog.Run()
}
