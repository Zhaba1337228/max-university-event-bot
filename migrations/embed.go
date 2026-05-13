// Package migrations предоставляет встроенный набор SQL-миграций для goose.
//
// Файлы *.sql встроены в бинарь через embed.FS, поэтому миграции работают
// без зависимости от рабочей директории. Тот же FS используется в проде
// (cmd/migrate) и в интеграционных тестах.
package migrations

import "embed"

// FS содержит все .sql-файлы из директории migrations/.
// Используется в cmd/migrate/main.go и (в будущем) в integration-тестах.
//
//go:embed *.sql
var FS embed.FS
