package ports

import "context"

type SecretStore interface {
	Set(context.Context, string, string) error
	Get(context.Context, string) (string, bool, error)
	Has(context.Context, string) (bool, error)
	Delete(context.Context, string) error
}
