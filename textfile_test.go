package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsTextFiles(t *testing.T) {
	dir := t.TempDir()

	// A UTF-8 byte-order mark and CRLF line endings are read and kept.
	p := filepath.Join(dir, "bom.ini")
	os.WriteFile(p, []byte("\xEF\xBB\xBF; Settings\r\n\r\n[net]\r\nport=1\r\n"), 0o644)
	if code, out, errOut := run(t, p, ".", "get", "net.port"); code != 0 || out != "1\n" {
		t.Fatalf("read with BOM: %d %q %q", code, out, errOut)
	}
	if code, _, errOut := run(t, p, ".", "set", "net.port=2", "net.host=x"); code != 0 {
		t.Fatalf("set: %d %q", code, errOut)
	}
	got, _ := os.ReadFile(p)
	want := "\xEF\xBB\xBF; Settings\r\n\r\n[net]\r\nport=2\r\nhost=x\r\n"
	if string(got) != want {
		t.Errorf("saved:\n got %q\nwant %q", got, want)
	}

	// LF stays LF, with no byte-order mark added.
	q := filepath.Join(dir, "lf.ini")
	os.WriteFile(q, []byte("a=1\n"), 0o644)
	run(t, q, ".", "set", "b=2")
	if got, _ := os.ReadFile(q); string(got) != "a=1\nb=2\n" {
		t.Errorf("LF file: %q", got)
	}

	// UTF-16 is refused plainly, as a file problem rather than line 1.
	for name, data := range map[string][]byte{
		"le.ini":    {0xFF, 0xFE, 'a', 0, '=', 0, '1', 0},
		"be.ini":    {0xFE, 0xFF, 0, 'a', 0, '=', 0, '1'},
		"nobom.ini": {'a', 0, '=', 0, '1', 0},
	} {
		f := filepath.Join(dir, name)
		os.WriteFile(f, data, 0o644)
		code, _, errOut := run(t, f, ".", "list")
		if code != exitSyntax || !strings.Contains(errOut, "UTF-16") || strings.Contains(errOut, "line 1") {
			t.Errorf("%s: %d %q", name, code, errOut)
		}
	}

	// A new file gets the platform's line endings (LF here).
	n := filepath.Join(dir, "new.ini")
	run(t, n, ".", "set", "a=1")
	if got, _ := os.ReadFile(n); !bytes.Equal(got, []byte("a=1\n")) && !bytes.Equal(got, []byte("a=1\r\n")) {
		t.Errorf("new file: %q", got)
	}

	// A schema with a byte-order mark loads too.
	s := filepath.Join(dir, "schema.ini")
	os.WriteFile(s, []byte("\xEF\xBB\xBF[@rule net.port]\r\ntype=port\r\n"), 0o644)
	if _, err := LoadSchema(s); err != nil {
		t.Errorf("schema with BOM: %v", err)
	}
}
