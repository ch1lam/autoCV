package logging

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const redactedValue = "[REDACTED]"

var userTextKeys = map[string]struct{}{
	"answer":      {},
	"content":     {},
	"document":    {},
	"jd":          {},
	"jd_text":     {},
	"markdown":    {},
	"prompt":      {},
	"quote":       {},
	"raw_text":    {},
	"resume":      {},
	"resume_text": {},
	"source_text": {},
	"text":        {},
}

type FileLogger struct {
	Logger *slog.Logger
	file   *os.File
}

func NewFile(path string, level slog.Leveler) (*FileLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("set log permissions: %w", err)
	}

	handler := NewRedactingHandler(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: level,
	}))
	return &FileLogger{
		Logger: slog.New(handler),
		file:   file,
	}, nil
}

func (logger *FileLogger) Close() error {
	return logger.file.Close()
}

func NewRedactingHandler(next slog.Handler) slog.Handler {
	return redactingHandler{next: next}
}

type redactingHandler struct {
	next slog.Handler
}

func (handler redactingHandler) Enabled(
	ctx context.Context,
	level slog.Level,
) bool {
	return handler.next.Enabled(ctx, level)
}

func (handler redactingHandler) Handle(
	ctx context.Context,
	record slog.Record,
) error {
	redacted := slog.NewRecord(
		record.Time,
		record.Level,
		record.Message,
		record.PC,
	)
	record.Attrs(func(attribute slog.Attr) bool {
		redacted.AddAttrs(redactAttribute(attribute))
		return true
	})
	return handler.next.Handle(ctx, redacted)
}

func (handler redactingHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attributes))
	for _, attribute := range attributes {
		redacted = append(redacted, redactAttribute(attribute))
	}
	return redactingHandler{next: handler.next.WithAttrs(redacted)}
}

func (handler redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{next: handler.next.WithGroup(name)}
}

func redactAttribute(attribute slog.Attr) slog.Attr {
	attribute.Value = attribute.Value.Resolve()
	if attribute.Value.Kind() == slog.KindGroup {
		group := attribute.Value.Group()
		redacted := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			redacted = append(redacted, redactAttribute(child))
		}
		return slog.Attr{
			Key:   attribute.Key,
			Value: slog.GroupValue(redacted...),
		}
	}

	key := normalizeKey(attribute.Key)
	if isSecretKey(key) {
		return slog.String(attribute.Key, redactedValue)
	}
	if _, sensitive := userTextKeys[key]; sensitive {
		if attribute.Value.Kind() != slog.KindString {
			return slog.String(attribute.Key, redactedValue)
		}
		return summarizeText(attribute.Key, attribute.Value.String())
	}
	return attribute
}

func normalizeKey(key string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(key))
}

func isSecretKey(key string) bool {
	for _, fragment := range []string{
		"access_token",
		"accesstoken",
		"api_key",
		"apikey",
		"authorization",
		"credential",
		"password",
		"refresh_token",
		"refreshtoken",
		"secret",
	} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return key == "token"
}

func summarizeText(key, value string) slog.Attr {
	digest := sha256.Sum256([]byte(value))
	return slog.Group(
		key,
		slog.Int("length", utf8.RuneCountInString(value)),
		slog.String("sha256", hex.EncodeToString(digest[:])),
	)
}
