package migrations

import "embed"

// FS contains the ordered database migrations applied at application startup.
//
//go:embed *.sql
var FS embed.FS
