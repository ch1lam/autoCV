package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactingHandlerSummarizesUserTextAndSecrets(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil)))

	logger.Info(
		"provider.request",
		slog.String("provider", "fake"),
		slog.Int("token_usage", 42),
		slog.String("resume", "private resume text"),
		slog.String("prompt", "private prompt"),
		slog.String("api_key", "secret-key"),
	)

	line := output.String()
	for _, privateValue := range []string{
		"private resume text",
		"private prompt",
		"secret-key",
	} {
		if strings.Contains(line, privateValue) {
			t.Fatalf("log contains private value %q: %s", privateValue, line)
		}
	}

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if entry["provider"] != "fake" {
		t.Fatalf("expected provider metadata, got %#v", entry["provider"])
	}
	if entry["token_usage"] != float64(42) {
		t.Fatalf("expected token usage metadata, got %#v", entry["token_usage"])
	}
	if entry["api_key"] != redactedValue {
		t.Fatalf("expected redacted API key, got %#v", entry["api_key"])
	}
	assertTextSummary(t, entry["resume"], len([]rune("private resume text")))
	assertTextSummary(t, entry["prompt"], len([]rune("private prompt")))
}

func TestRedactingHandlerRedactsNestedGroups(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil)))

	logger.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"resume.generated",
		slog.Group(
			"artifact",
			slog.String("content", "private markdown"),
			slog.String("content_hash", "already-safe"),
		),
	)

	if strings.Contains(output.String(), "private markdown") {
		t.Fatalf("log contains private grouped content: %s", output.String())
	}

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	artifact := entry["artifact"].(map[string]any)
	assertTextSummary(t, artifact["content"], len([]rune("private markdown")))
	if artifact["content_hash"] != "already-safe" {
		t.Fatalf("expected existing content hash to remain visible")
	}
}

func TestFileLoggerUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "autocv.log")
	logger, err := NewFile(path, slog.LevelInfo)
	if err != nil {
		t.Fatalf("create file logger: %v", err)
	}
	logger.Logger.Info("application.start")
	if err := logger.Close(); err != nil {
		t.Fatalf("close file logger: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected log mode 0600, got %o", info.Mode().Perm())
	}
}

func assertTextSummary(t *testing.T, value any, expectedLength int) {
	t.Helper()

	summary, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected text summary object, got %#v", value)
	}
	if summary["length"] != float64(expectedLength) {
		t.Fatalf("expected length %d, got %#v", expectedLength, summary["length"])
	}
	digest, ok := summary["sha256"].(string)
	if !ok || len(digest) != 64 {
		t.Fatalf("expected SHA-256 digest, got %#v", summary["sha256"])
	}
}
