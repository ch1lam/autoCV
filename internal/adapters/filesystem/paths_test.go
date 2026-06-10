package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsEnsureCreatesManagedDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "autocv-data")
	paths, err := NewPaths(root)
	if err != nil {
		t.Fatalf("create paths: %v", err)
	}

	if err := paths.Ensure(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}

	for _, directory := range []string{
		paths.Root,
		paths.Sources,
		paths.Runs,
		paths.Exports,
		paths.Logs,
		paths.Backups,
	} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("stat %s: %v", directory, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", directory)
		}
	}

	if paths.Database != filepath.Join(root, "autocv.db") {
		t.Fatalf("unexpected database path %q", paths.Database)
	}
	if paths.Config != filepath.Join(root, "config.json") {
		t.Fatalf("unexpected config path %q", paths.Config)
	}
}

func TestDefaultPathsUsesEnvironmentOverride(t *testing.T) {
	root := filepath.Join(t.TempDir(), "override")
	t.Setenv(dataDirectoryEnv, root)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("resolve default paths: %v", err)
	}
	if paths.Root != root {
		t.Fatalf("expected root %q, got %q", root, paths.Root)
	}
}
