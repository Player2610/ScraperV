// Package db provides embedded SQL migrations for use with goose.
package db

import "embed"

// Migrations holds all SQL migration files.
//
//go:embed migrations/*.sql
var Migrations embed.FS
