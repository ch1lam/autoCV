package keychain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ch1lam/autocv/internal/ports"
)

const securityPath = "/usr/bin/security"

type commandRunner interface {
	Run(
		context.Context,
		string,
		...string,
	) (string, string, error)
}

type execRunner struct{}

func (execRunner) Run(
	ctx context.Context,
	stdin string,
	args ...string,
) (string, string, error) {
	command := exec.CommandContext(ctx, securityPath, args...)
	command.Stdin = strings.NewReader(stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

type Store struct {
	service string
	runner  commandRunner
}

func New(service string) *Store {
	return &Store{
		service: service,
		runner:  execRunner{},
	}
}

func (store *Store) Set(
	ctx context.Context,
	reference string,
	secret string,
) error {
	if err := store.validate(reference); err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("secret is empty")
	}
	_, stderr, err := store.runner.Run(
		ctx,
		secret+"\n",
		"add-generic-password",
		"-U",
		"-a",
		reference,
		"-s",
		store.service,
		"-w",
	)
	if err != nil {
		return securityError("save keychain secret", stderr, err)
	}
	return nil
}

func (store *Store) Get(
	ctx context.Context,
	reference string,
) (string, bool, error) {
	if err := store.validate(reference); err != nil {
		return "", false, err
	}
	stdout, stderr, err := store.runner.Run(
		ctx,
		"",
		"find-generic-password",
		"-a",
		reference,
		"-s",
		store.service,
		"-w",
	)
	if isItemNotFound(err, stderr) {
		return "", false, nil
	}
	if err != nil {
		return "", false, securityError(
			"read keychain secret",
			stderr,
			err,
		)
	}
	return strings.TrimSuffix(stdout, "\n"), true, nil
}

func (store *Store) Has(
	ctx context.Context,
	reference string,
) (bool, error) {
	if err := store.validate(reference); err != nil {
		return false, err
	}
	_, stderr, err := store.runner.Run(
		ctx,
		"",
		"find-generic-password",
		"-a",
		reference,
		"-s",
		store.service,
	)
	if isItemNotFound(err, stderr) {
		return false, nil
	}
	if err != nil {
		return false, securityError(
			"check keychain secret",
			stderr,
			err,
		)
	}
	return true, nil
}

func (store *Store) Delete(
	ctx context.Context,
	reference string,
) error {
	if err := store.validate(reference); err != nil {
		return err
	}
	_, stderr, err := store.runner.Run(
		ctx,
		"",
		"delete-generic-password",
		"-a",
		reference,
		"-s",
		store.service,
	)
	if isItemNotFound(err, stderr) {
		return nil
	}
	if err != nil {
		return securityError("delete keychain secret", stderr, err)
	}
	return nil
}

func (store *Store) validate(reference string) error {
	if strings.TrimSpace(store.service) == "" {
		return errors.New("keychain service is empty")
	}
	if strings.TrimSpace(reference) == "" {
		return errors.New("keychain reference is empty")
	}
	return nil
}

func securityError(operation string, stderr string, err error) error {
	message := strings.TrimSpace(stderr)
	if message == "" {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: %s: %w", operation, message, err)
}

func isItemNotFound(err error, stderr string) bool {
	if err == nil {
		return false
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 44 {
		return true
	}
	message := strings.ToLower(stderr)
	return strings.Contains(message, "could not be found") ||
		strings.Contains(message, "not found")
}

var _ ports.SecretStore = (*Store)(nil)
