package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripKeepsCommentsAndOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	in := "# header\n\nB=2\nA='with space'\nC=\"x \\\"q\\\"\"\n"
	if err := os.WriteFile(path, []byte(in), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Get("A"); got != "with space" {
		t.Fatalf("A = %q", got)
	}
	if got := f.Get("C"); got != `x "q"` {
		t.Fatalf("C = %q", got)
	}
	f.Set("B", "3")
	f.Set("D", "p$ss #1")
	f.SetDefault("B", "ignored")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	want := "# header\n\nB=3\nA='with space'\nC='x \"q\"'\nD='p$ss #1'\n"
	if string(out) != want {
		t.Fatalf("saved:\n%s\nwant:\n%s", out, want)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
	again, _ := Load(path)
	if again.Get("D") != "p$ss #1" {
		t.Fatalf("D = %q", again.Get("D"))
	}
}

func TestQuoteFallsBackToDoubleQuotes(t *testing.T) {
	v := `it's "x"`
	if got := unquote(quote(v)); got != v {
		t.Fatalf("round trip = %q", got)
	}
}

func TestDelete(t *testing.T) {
	f := New("")
	f.Set("A", "1")
	f.Set("B", "2")
	f.Delete("A")
	f.Delete("missing")
	if _, ok := f.Lookup("A"); ok || f.Get("B") != "2" || len(f.Keys()) != 1 {
		t.Fatalf("keys after delete: %v", f.Keys())
	}
}
