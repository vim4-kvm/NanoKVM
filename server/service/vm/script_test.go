package vm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScriptNamesAreArgumentsNotShellCode(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a script.sh", "a;exit 9;.sh", "quote'file.sh"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("printf 'script-ok'"), 0700); err != nil {
			t.Fatal(err)
		}
		cmd, err := scriptCommand(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		output, err := cmd.CombinedOutput()
		if err != nil || string(output) != "script-ok" {
			t.Fatalf("%q: %q %v", name, output, err)
		}
	}
	for _, name := range []string{"../outside.sh", "/tmp/outside.sh", "file.txt", "x\\y.sh"} {
		if _, err := scriptPath(dir, name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "link.sh")); err != nil {
		t.Fatal(err)
	}
	if _, err := scriptCommand(dir, "link.sh"); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestScriptHonorsShebang(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bash.sh"), []byte("#!/bin/bash\na=(one two); printf '%s' \"${a[1]}\""), 0700); err != nil {
		t.Fatal(err)
	}
	cmd, err := scriptCommand(dir, "bash.sh")
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != "two" {
		t.Fatalf("%s: %v", output, err)
	}
}
