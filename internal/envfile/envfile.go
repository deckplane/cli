// Package envfile reads and writes .env files while keeping their order,
// comments and blank lines, so files stay readable after the CLI edits them.
package envfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type line struct {
	key   string // empty for comments and blank lines
	value string
	raw   string
}

// File is an ordered .env file.
type File struct {
	Path  string
	lines []line
}

// New returns an empty file that will be written to path.
func New(path string) *File { return &File{Path: path} }

// Load reads path. A missing file is returned as an error (use os.IsNotExist).
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &File{Path: path}
	for _, raw := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			f.lines = append(f.lines, line{raw: raw})
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if !ok {
			f.lines = append(f.lines, line{raw: raw})
			continue
		}
		f.lines = append(f.lines, line{key: strings.TrimSpace(k), value: unquote(strings.TrimSpace(v))})
	}
	return f, nil
}

// Get returns the value for key, or "" when missing.
func (f *File) Get(key string) string {
	v, _ := f.Lookup(key)
	return v
}

// Lookup returns the value for key and whether it exists.
func (f *File) Lookup(key string) (string, bool) {
	for _, l := range f.lines {
		if l.key == key {
			return l.value, true
		}
	}
	return "", false
}

// Set updates key in place, or appends it at the end.
func (f *File) Set(key, value string) {
	for i, l := range f.lines {
		if l.key == key {
			f.lines[i].value = value
			return
		}
	}
	f.lines = append(f.lines, line{key: key, value: value})
}

// SetDefault sets key only when it is missing or empty.
func (f *File) SetDefault(key, value string) {
	if f.Get(key) == "" {
		f.Set(key, value)
	}
}

// Comment appends a comment line (a blank line precedes it unless the file is empty).
func (f *File) Comment(text string) {
	if len(f.lines) > 0 {
		f.lines = append(f.lines, line{raw: ""})
	}
	f.lines = append(f.lines, line{raw: "# " + text})
}

// Keys returns all keys in file order.
func (f *File) Keys() []string {
	var keys []string
	for _, l := range f.lines {
		if l.key != "" {
			keys = append(keys, l.key)
		}
	}
	return keys
}

// Save writes the file atomically with owner-only permissions, because it
// holds secrets.
func (f *File) Save() error {
	var b strings.Builder
	for _, l := range f.lines {
		if l.key == "" {
			b.WriteString(l.raw)
		} else {
			fmt.Fprintf(&b, "%s=%s", l.key, quote(l.value))
		}
		b.WriteByte('\n')
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".env-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.Path)
}

// quote wraps values that need it. Single quotes are literal in Docker
// Compose env files (no $ interpolation), so they are preferred.
func quote(v string) string {
	if !strings.ContainsAny(v, " \t#\"'$\\") {
		return v
	}
	if !strings.Contains(v, "'") {
		return "'" + v + "'"
	}
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return `"` + v + `"`
}

func unquote(v string) string {
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return v[1 : len(v)-1]
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
		v = strings.ReplaceAll(v, `\"`, `"`)
		return strings.ReplaceAll(v, `\\`, `\`)
	}
	return v
}
