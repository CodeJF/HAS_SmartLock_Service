package db

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	migrationfiles "has-smartlock-service/migrations"
)

type migrationFile struct {
	Version int
	Name    string
	Body    string
}

type schemaMigration struct {
	Version   int       `gorm:"column:version"`
	Name      string    `gorm:"column:name"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

func RunMigrations(database *gorm.DB) error {
	if err := ensureSchemaMigrationsTable(database); err != nil {
		return err
	}

	files, err := loadMigrationFiles()
	if err != nil {
		return err
	}

	applied, err := loadAppliedVersions(database)
	if err != nil {
		return err
	}

	for _, file := range files {
		if applied[file.Version] {
			continue
		}

		if err := database.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(file.Body).Error; err != nil {
				return fmt.Errorf("run migration %s: %w", file.Name, err)
			}
			return tx.Exec(
				"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
				file.Version,
				file.Name,
				time.Now(),
			).Error
		}); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaMigrationsTable(database *gorm.DB) error {
	return database.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INT NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
)`).Error
}

func loadAppliedVersions(database *gorm.DB) (map[int]bool, error) {
	var rows []schemaMigration
	if err := database.Raw("SELECT version, name, applied_at FROM schema_migrations").Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[int]bool, len(rows))
	for _, row := range rows {
		result[row.Version] = true
	}
	return result, nil
}

func loadMigrationFiles() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrationfiles.FS, ".")
	if err != nil {
		return nil, err
	}

	result := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		if !isVersionedMigrationFile(entry.Name()) {
			continue
		}

		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}

		body, err := migrationfiles.FS.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}

		result = append(result, migrationFile{
			Version: version,
			Name:    entry.Name(),
			Body:    string(body),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func isVersionedMigrationFile(name string) bool {
	prefix, _, found := strings.Cut(name, "_")
	if !found || prefix == "" {
		return false
	}

	for _, r := range prefix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseMigrationVersion(name string) (int, error) {
	prefix, _, found := strings.Cut(name, "_")
	if !found {
		return 0, fmt.Errorf("invalid migration filename %q", name)
	}

	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("invalid migration version in %q: %w", name, err)
	}
	return version, nil
}
