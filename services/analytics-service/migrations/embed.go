// Package migrations embeds the ClickHouse DDL source of truth (*.sql)
// into the service binary, so the migration runner works regardless of
// the process working directory (Docker runtime, Kubernetes, local runs).
package migrations

import "embed"

// FS holds all migration files in version order (001_*.sql ... 014_*.sql).
//
//go:embed *.sql
var FS embed.FS
