package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedFilesSaveReadAndDelete(t *testing.T) {
	root := t.TempDir()
	store, err := NewManagedFiles(root)
	if err != nil {
		t.Fatalf("create managed file store: %v", err)
	}
	contents := []byte("# Profile\n\nPrivate career content.")

	managedPath, err := store.SaveMarkdown(
		"profile-1",
		"document-1",
		contents,
	)
	if err != nil {
		t.Fatalf("save Markdown: %v", err)
	}
	expectedPath := "sources/profile-1/document-1/source.md"
	if managedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, managedPath)
	}

	actual, err := store.Read(managedPath)
	if err != nil {
		t.Fatalf("read Markdown: %v", err)
	}
	if string(actual) != string(contents) {
		t.Fatalf("managed contents differ")
	}

	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(managedPath)))
	if err != nil {
		t.Fatalf("stat managed file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}

	if err := store.Delete(managedPath); err != nil {
		t.Fatalf("delete managed file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(managedPath))); !os.IsNotExist(err) {
		t.Fatalf("expected managed file to be deleted, got %v", err)
	}
}

func TestManagedFilesSavesDOCX(t *testing.T) {
	root := t.TempDir()
	store, err := NewManagedFiles(root)
	if err != nil {
		t.Fatalf("create managed file store: %v", err)
	}
	contents := []byte("PK\x03\x04synthetic docx")

	managedPath, err := store.SaveDOCX(
		"profile-1",
		"document-1",
		contents,
	)
	if err != nil {
		t.Fatalf("save DOCX: %v", err)
	}
	expectedPath := "sources/profile-1/document-1/source.docx"
	if managedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, managedPath)
	}

	actual, err := store.Read(managedPath)
	if err != nil {
		t.Fatalf("read DOCX: %v", err)
	}
	if string(actual) != string(contents) {
		t.Fatalf("managed DOCX differs")
	}
}

func TestManagedFilesRejectsPathTraversal(t *testing.T) {
	store, err := NewManagedFiles(t.TempDir())
	if err != nil {
		t.Fatalf("create managed file store: %v", err)
	}

	for _, path := range []string{
		"../private.md",
		"/tmp/private.md",
		"sources/../../private.md",
	} {
		if _, err := store.Read(path); err == nil {
			t.Fatalf("expected path %q to be rejected", path)
		}
	}
}

func TestManagedFilesRejectsUnsafeIDs(t *testing.T) {
	store, err := NewManagedFiles(t.TempDir())
	if err != nil {
		t.Fatalf("create managed file store: %v", err)
	}

	if _, err := store.SaveMarkdown("../profile", "document", nil); err == nil {
		t.Fatal("expected unsafe profile id error")
	}
	if _, err := store.SaveMarkdown("profile", "nested/document", nil); err == nil {
		t.Fatal("expected unsafe document id error")
	}
	if _, err := store.SaveDOCX("../profile", "document", nil); err == nil {
		t.Fatal("expected unsafe DOCX profile id error")
	}
	if _, err := store.SaveDOCX("profile", "nested/document", nil); err == nil {
		t.Fatal("expected unsafe DOCX document id error")
	}
}

func TestManagedFilesSavesAndExportsArtifactAtomically(t *testing.T) {
	root := t.TempDir()
	store, err := NewManagedFiles(root)
	if err != nil {
		t.Fatalf("create managed file store: %v", err)
	}
	contents := []byte("%PDF-1.7\nsynthetic")

	managedPath, err := store.SaveArtifact(
		"run-1",
		"artifact-1",
		"pdf",
		contents,
	)
	if err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if managedPath != "runs/run-1/artifacts/artifact-1.pdf" {
		t.Fatalf("unexpected artifact path %q", managedPath)
	}

	exportPath := filepath.Join(t.TempDir(), "resume.pdf")
	if err := store.ExportArtifact(managedPath, exportPath); err != nil {
		t.Fatalf("export artifact: %v", err)
	}
	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read exported artifact: %v", err)
	}
	if string(exported) != string(contents) {
		t.Fatal("exported artifact differs from managed artifact")
	}
}
