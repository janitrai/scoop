package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectJSONFilesRecursive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.json"), `{"k":"v"}`)
	mustWriteFile(t, filepath.Join(root, "b.txt"), `x`)
	mustWriteFile(t, filepath.Join(root, ".hidden.json"), `{}`)
	mustWriteFile(t, filepath.Join(root, "nested", "c.json"), `{"k":"v2"}`)

	files, err := collectJSONFiles(root, true)
	if err != nil {
		t.Fatalf("collectJSONFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 json files, got %d (%v)", len(files), files)
	}
}

func TestCollectJSONFilesNonRecursive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.json"), `{"k":"v"}`)
	mustWriteFile(t, filepath.Join(root, "nested", "c.json"), `{"k":"v2"}`)

	files, err := collectJSONFiles(root, false)
	if err != nil {
		t.Fatalf("collectJSONFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 json file, got %d (%v)", len(files), files)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
