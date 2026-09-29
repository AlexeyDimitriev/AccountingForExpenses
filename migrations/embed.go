package migrations

import "embed"

// Files содержит SQL-миграции в порядке их числовых префиксов.
//
//go:embed *.sql
var Files embed.FS
