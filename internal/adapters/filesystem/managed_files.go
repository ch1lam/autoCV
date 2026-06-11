package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ch1lam/autocv/internal/ports"
)

const managedMarkdownName = "source.md"

type ManagedFiles struct {
	root string
}

func NewManagedFiles(root string) (*ManagedFiles, error) {
	if root == "" {
		return nil, errors.New("managed file root is empty")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve managed file root: %w", err)
	}
	return &ManagedFiles{root: absoluteRoot}, nil
}

func (store *ManagedFiles) SaveMarkdown(
	profileID string,
	documentID string,
	contents []byte,
) (string, error) {
	if err := validatePathID(profileID); err != nil {
		return "", fmt.Errorf("invalid profile id: %w", err)
	}
	if err := validatePathID(documentID); err != nil {
		return "", fmt.Errorf("invalid document id: %w", err)
	}

	relativePath := filepath.Join(
		"sources",
		profileID,
		documentID,
		managedMarkdownName,
	)
	absolutePath, err := store.resolve(relativePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o700); err != nil {
		return "", fmt.Errorf("create managed document directory: %w", err)
	}

	temporary, err := os.CreateTemp(
		filepath.Dir(absolutePath),
		".source-*.tmp",
	)
	if err != nil {
		return "", fmt.Errorf("create managed document temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", fmt.Errorf("set managed document permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write managed document: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", fmt.Errorf("sync managed document: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close managed document: %w", err)
	}
	if err := os.Rename(temporaryPath, absolutePath); err != nil {
		return "", fmt.Errorf("replace managed document: %w", err)
	}
	return filepath.ToSlash(relativePath), nil
}

func (store *ManagedFiles) Read(relativePath string) ([]byte, error) {
	absolutePath, err := store.resolve(relativePath)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("read managed file: %w", err)
	}
	return contents, nil
}

func (store *ManagedFiles) Delete(relativePath string) error {
	absolutePath, err := store.resolve(relativePath)
	if err != nil {
		return err
	}
	if err := os.Remove(absolutePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete managed file: %w", err)
	}

	documentDirectory := filepath.Dir(absolutePath)
	if err := os.Remove(documentDirectory); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete managed document directory: %w", err)
	}
	return nil
}

func (store *ManagedFiles) resolve(relativePath string) (string, error) {
	if relativePath == "" || filepath.IsAbs(relativePath) {
		return "", errors.New("managed path must be relative")
	}
	cleanPath := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanPath == "." ||
		cleanPath == ".." ||
		strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", errors.New("managed path escapes application data directory")
	}

	absolutePath := filepath.Join(store.root, cleanPath)
	relativeToRoot, err := filepath.Rel(store.root, absolutePath)
	if err != nil {
		return "", fmt.Errorf("validate managed path: %w", err)
	}
	if relativeToRoot == ".." ||
		strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("managed path escapes application data directory")
	}
	return absolutePath, nil
}

func validatePathID(value string) error {
	if value == "" || value == "." || value == ".." {
		return errors.New("id is empty or reserved")
	}
	if filepath.Base(value) != value ||
		strings.ContainsAny(value, `/\`) {
		return errors.New("id contains a path separator")
	}
	return nil
}

var _ ports.ManagedFileStore = (*ManagedFiles)(nil)
