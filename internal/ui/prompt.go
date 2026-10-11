package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ErrNotInteractive is returned by prompts when stdin is not a terminal.
var ErrNotInteractive = errors.New("cannot ask a question: not running in an interactive terminal")

var stdin = bufio.NewReader(os.Stdin)

// Interactive reports whether we can ask the user questions.
func Interactive() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

func readLine() (string, error) {
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Ask asks for a line of text. An empty answer returns def.
func Ask(label, def string) (string, error) {
	if !Interactive() {
		return "", ErrNotInteractive
	}
	if def != "" {
		fmt.Fprintf(Out, "%s %s %s ", Cyan("?"), label, Dim("("+def+")"))
	} else {
		fmt.Fprintf(Out, "%s %s ", Cyan("?"), label)
	}
	answer, err := readLine()
	if err != nil {
		return "", err
	}
	if answer == "" {
		return def, nil
	}
	return answer, nil
}

// AskRequired keeps asking until validate accepts the answer.
func AskRequired(label, def string, validate func(string) error) (string, error) {
	for {
		answer, err := Ask(label, def)
		if err != nil {
			return "", err
		}
		if validate == nil {
			if answer != "" {
				return answer, nil
			}
			Warn("A value is required.")
			continue
		}
		if verr := validate(answer); verr != nil {
			Warn("%s", verr.Error())
			continue
		}
		return answer, nil
	}
}

// AskSecret reads a value without echoing it, keeps asking until validate
// accepts it.
func AskSecret(label string, validate func(string) error) (string, error) {
	if !Interactive() {
		return "", ErrNotInteractive
	}
	for {
		fmt.Fprintf(Out, "%s %s ", Cyan("?"), label)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(Out)
		if err != nil {
			return "", err
		}
		answer := strings.TrimSpace(string(raw))
		if validate != nil {
			if verr := validate(answer); verr != nil {
				Warn("%s", verr.Error())
				continue
			}
		}
		return answer, nil
	}
}

// Confirm asks a yes/no question.
func Confirm(label string, def bool) (bool, error) {
	if !Interactive() {
		return false, ErrNotInteractive
	}
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	for {
		fmt.Fprintf(Out, "%s %s %s ", Cyan("?"), label, Dim("("+hint+")"))
		answer, err := readLine()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(answer) {
		case "":
			return def, nil
		case "y", "yes", "e", "evet":
			return true, nil
		case "n", "no", "h", "hayir", "hayır":
			return false, nil
		}
		Warn("Please answer y or n.")
	}
}

// Choose shows a numbered list and returns the index the user picked.
func Choose(label string, options []string, def int) (int, error) {
	if !Interactive() {
		return 0, ErrNotInteractive
	}
	fmt.Fprintf(Out, "%s %s\n", Cyan("?"), label)
	for i, opt := range options {
		marker := " "
		if i == def {
			marker = Cyan("›")
		}
		fmt.Fprintf(Out, "  %s %d) %s\n", marker, i+1, opt)
	}
	for {
		fmt.Fprintf(Out, "  %s ", Dim(fmt.Sprintf("Enter a number (%d):", def+1)))
		answer, err := readLine()
		if err != nil {
			return 0, err
		}
		if answer == "" {
			return def, nil
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		Warn("Enter a number between 1 and %d.", len(options))
	}
}
