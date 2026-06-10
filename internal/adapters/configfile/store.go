package configfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const CurrentVersion = 1

type Config struct {
	Version         int    `json:"version"`
	DefaultLanguage string `json:"defaultLanguage"`
}

type Store struct {
	path string
}

func New(path string) Store {
	return Store{path: path}
}

func Default() Config {
	return Config{
		Version:         CurrentVersion,
		DefaultLanguage: "zh-CN",
	}
}

func (store Store) LoadOrCreate() (Config, error) {
	config, err := store.Load()
	if err == nil {
		return config, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	config = Default()
	if err := store.Save(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (store Store) Load() (Config, error) {
	contents, err := os.ReadFile(store.path)
	if err != nil {
		return Config{}, err
	}

	var config Config
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("decode config: trailing content")
	}
	if err := validate(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (store Store) Save(config Config) error {
	if err := validate(config); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	contents = append(contents, '\n')

	temporary, err := os.CreateTemp(filepath.Dir(store.path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(temporaryPath, store.path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func validate(config Config) error {
	if config.Version != CurrentVersion {
		return fmt.Errorf(
			"unsupported config version %d, expected %d",
			config.Version,
			CurrentVersion,
		)
	}
	if config.DefaultLanguage == "" {
		return errors.New("default language is empty")
	}
	return nil
}
