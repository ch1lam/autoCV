package keychain

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type runnerCall struct {
	stdin string
	args  []string
}

type fakeRunner struct {
	calls  []runnerCall
	stdout string
	stderr string
	err    error
}

func (runner *fakeRunner) Run(
	_ context.Context,
	stdin string,
	args ...string,
) (string, string, error) {
	runner.calls = append(runner.calls, runnerCall{
		stdin: stdin,
		args:  append([]string(nil), args...),
	})
	return runner.stdout, runner.stderr, runner.err
}

func TestStorePassesSecretThroughStdin(t *testing.T) {
	runner := &fakeRunner{}
	store := &Store{
		service: "io.github.ch1lam.autocv",
		runner:  runner,
	}
	if err := store.Set(
		context.Background(),
		"openai-api-key",
		"sk-test-secret",
	); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected one command, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.stdin != "sk-test-secret\n" {
		t.Fatalf("unexpected stdin %q", call.stdin)
	}
	expected := []string{
		"add-generic-password",
		"-U",
		"-a",
		"openai-api-key",
		"-s",
		"io.github.ch1lam.autocv",
		"-w",
	}
	if !reflect.DeepEqual(call.args, expected) {
		t.Fatalf("unexpected args %#v", call.args)
	}
	for _, arg := range call.args {
		if arg == "sk-test-secret" {
			t.Fatal("secret must not be placed in process arguments")
		}
	}
}

func TestStoreReadsAndChecksSecretWithoutLeakingIt(t *testing.T) {
	runner := &fakeRunner{stdout: "sk-saved\n"}
	store := &Store{
		service: "io.github.ch1lam.autocv",
		runner:  runner,
	}
	secret, found, err := store.Get(
		context.Background(),
		"openai-api-key",
	)
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if !found || secret != "sk-saved" {
		t.Fatalf("unexpected secret result found=%v secret=%q", found, secret)
	}

	runner.stdout = ""
	has, err := store.Has(context.Background(), "openai-api-key")
	if err != nil {
		t.Fatalf("check secret: %v", err)
	}
	if !has {
		t.Fatal("expected secret to exist")
	}
	if got := runner.calls[1].args[len(runner.calls[1].args)-1]; got == "-w" {
		t.Fatal("Has must not request the secret value")
	}
}

func TestStoreTreatsMissingSecretAsAbsent(t *testing.T) {
	runner := &fakeRunner{
		stderr: "security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain.",
		err:    errors.New("exit status 44"),
	}
	store := &Store{
		service: "io.github.ch1lam.autocv",
		runner:  runner,
	}
	_, found, err := store.Get(context.Background(), "openai-api-key")
	if err != nil {
		t.Fatalf("get missing secret: %v", err)
	}
	if found {
		t.Fatal("expected missing secret")
	}
}
