package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const (
	applicationDirectory = "AutoCV"
	dataDirectoryEnv     = "AUTOCV_DATA_DIR"
)

type Paths struct {
	Root     string
	Database string
	Config   string
	Sources  string
	Runs     string
	Exports  string
	Logs     string
	Backups  string
}

func DefaultPaths() (Paths, error) {
	if override := os.Getenv(dataDirectoryEnv); override != "" {
		return NewPaths(override)
	}

	root, err := defaultRoot()
	if err != nil {
		return Paths{}, err
	}
	return NewPaths(root)
}

func NewPaths(root string) (Paths, error) {
	if root == "" {
		return Paths{}, errors.New("application data directory is empty")
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}

	return Paths{
		Root:     absoluteRoot,
		Database: filepath.Join(absoluteRoot, "autocv.db"),
		Config:   filepath.Join(absoluteRoot, "config.json"),
		Sources:  filepath.Join(absoluteRoot, "sources"),
		Runs:     filepath.Join(absoluteRoot, "runs"),
		Exports:  filepath.Join(absoluteRoot, "exports"),
		Logs:     filepath.Join(absoluteRoot, "logs"),
		Backups:  filepath.Join(absoluteRoot, "backups"),
	}, nil
}

func (paths Paths) Ensure() error {
	for _, directory := range []string{
		paths.Root,
		paths.Sources,
		paths.Runs,
		paths.Exports,
		paths.Logs,
		paths.Backups,
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func defaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", applicationDirectory), nil
	case "windows":
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, applicationDirectory), nil
	default:
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			return filepath.Join(dataHome, applicationDirectory), nil
		}
		return filepath.Join(home, ".local", "share", applicationDirectory), nil
	}
}
