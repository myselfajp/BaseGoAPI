// Package migrations embeds the SQL migration files so they travel inside the
// compiled binary and can be applied at startup (the golang-migrate equivalent
// of running `alembic upgrade head`).
package migrations

import "embed"

// FS holds the embedded *.up.sql / *.down.sql migration files.
//
//go:embed *.sql
var FS embed.FS
