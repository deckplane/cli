// Package ui prints friendly, consistent terminal output and asks questions.
// Colors are disabled automatically when output is not a terminal or when
// NO_COLOR is set.
package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Out is where all normal output goes. Tests may replace it.
var Out io.Writer = os.Stdout

var (
	bold   = lipgloss.NewStyle().Bold(true)
	dim    = lipgloss.NewStyle().Faint(true)
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	box    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(0, 2)
)

func Bold(s string) string   { return bold.Render(s) }
func Dim(s string) string    { return dim.Render(s) }
func Green(s string) string  { return green.Render(s) }
func Red(s string) string    { return red.Render(s) }
func Yellow(s string) string { return yellow.Render(s) }
func Cyan(s string) string   { return cyan.Render(s) }

// Title prints a section heading.
func Title(s string) { fmt.Fprintf(Out, "\n%s\n\n", Bold(s)) }

// Step announces work that is about to start.
func Step(format string, args ...any) {
	fmt.Fprintf(Out, "%s %s\n", Cyan("→"), fmt.Sprintf(format, args...))
}

// Success reports work that finished.
func Success(format string, args ...any) {
	fmt.Fprintf(Out, "%s %s\n", Green("✓"), fmt.Sprintf(format, args...))
}

// Warn reports something the user should know about but that is not fatal.
func Warn(format string, args ...any) {
	fmt.Fprintf(Out, "%s %s\n", Yellow("!"), fmt.Sprintf(format, args...))
}

// Info prints an indented, dimmed detail line.
func Info(format string, args ...any) {
	fmt.Fprintf(Out, "  %s\n", Dim(fmt.Sprintf(format, args...)))
}

// Println prints a plain line.
func Println(format string, args ...any) { fmt.Fprintf(Out, format+"\n", args...) }

// Box prints lines inside a rounded frame, used for final summaries.
func Box(lines ...string) {
	fmt.Fprintf(Out, "\n%s\n\n", box.Render(strings.Join(lines, "\n")))
}

// Error is an error with a suggestion telling the user how to fix it.
type Error struct {
	Msg  string
	Hint string
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

// NewError builds an error with a hint. err may be nil.
func NewError(msg, hint string, err error) error {
	return &Error{Msg: msg, Hint: hint, Err: err}
}

// PrintError writes err to stderr, including its hint when present.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "\n%s %s\n", Red("✗"), err.Error())
	var e *Error
	if errors.As(err, &e) && e.Hint != "" {
		for _, line := range strings.Split(e.Hint, "\n") {
			fmt.Fprintf(os.Stderr, "  %s\n", line)
		}
	}
}
