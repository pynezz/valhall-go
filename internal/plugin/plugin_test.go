package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil { // umask-proof
		t.Fatal(err)
	}
	return p
}

func TestParseHeader(t *testing.T) {
	d := t.TempDir()
	p := write(t, d, "x.sh",
		"#!/bin/sh\n# stoker-plugin: My Screen\n# stoker-order: 42\n"+
			"# stoker-timeout: 7\n# stoker-root: yes\necho hi\n", 0o755)
	m := parseHeader(p)
	if m == nil || m.title != "My Screen" || m.order != 42 ||
		m.timeout != 7*time.Second || !m.root {
		t.Fatalf("bad meta: %+v", m)
	}
	if parseHeader(write(t, d, "no.sh", "#!/bin/sh\necho\n", 0o755)) != nil {
		t.Fatal("headerless script must not parse")
	}
}

func TestTrustRejectsWorldWritable(t *testing.T) {
	d := t.TempDir()
	p := write(t, d, "evil.sh", "# stoker-plugin: evil\n", 0o777)
	if ok, _ := trusted(p, os.Geteuid()); ok {
		t.Fatal("world-writable plugin must be rejected")
	}
	rep := LoadAll([]string{d}, os.Geteuid())
	if len(rep.Loaded) != 0 || len(rep.Skipped) == 0 {
		t.Fatalf("expected skip report, got %+v", rep)
	}
}

func TestLoadAllHappyPath(t *testing.T) {
	d := t.TempDir()
	_ = os.Chmod(d, 0o755)
	write(t, d, "ok.sh", "#!/bin/sh\n# stoker-plugin: OK Screen\necho\n", 0o755)
	rep := LoadAll([]string{d}, os.Geteuid())
	if len(rep.Loaded) != 1 || rep.Loaded[0] != "sh:OK Screen" {
		t.Fatalf("expected one loaded plugin, got %+v", rep)
	}
}
