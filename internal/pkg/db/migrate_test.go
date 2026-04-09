package db

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	gosqlmysql "github.com/go-sql-driver/mysql"

	"has-smartlock-service/internal/pkg/config"
)

func TestParseMigrationVersion(t *testing.T) {
	version, err := parseMigrationVersion("001_init_schema.sql")
	if err != nil {
		t.Fatalf("parseMigrationVersion returned error: %v", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	testDSN := prepareMigrationTestDatabase(t, "has_smartlock_service_migrate_test")

	database, err := Open(config.Config{DBDSN: testDSN})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := RunMigrations(database); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	var count int64
	if err := database.Table("schema_migrations").Count(&count).Error; err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count != 3 {
		t.Fatalf("schema_migrations count = %d, want 3", count)
	}

	for _, table := range []string{"users", "verification_codes", "refresh_tokens", "user_clients", "homes", "home_members", "devices", "home_devices", "home_share_invites"} {
		var exists int
		if err := database.Raw(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			table,
		).Scan(&exists).Error; err != nil {
			t.Fatalf("check table %s exists: %v", table, err)
		}
		if exists != 1 {
			t.Fatalf("table %s not created by migrations", table)
		}
	}
}

func prepareMigrationTestDatabase(t *testing.T, databaseName string) string {
	t.Helper()

	rootDSN := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if rootDSN == "" {
		rootDSN = strings.TrimSpace(config.Load().DBDSN)
	}
	if rootDSN == "" {
		t.Skip("TEST_MYSQL_DSN or MYSQL_DSN/DB_DSN is required for migration tests")
	}

	parsed, err := gosqlmysql.ParseDSN(rootDSN)
	if err != nil {
		t.Skipf("skip migration tests because configured DSN is not a valid MySQL DSN: %v", err)
	}

	adminCfg := *parsed
	adminCfg.DBName = ""
	adminCfg.MultiStatements = true
	if adminCfg.Loc == nil {
		adminCfg.Loc = time.Local
	}

	adminDB, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatalf("open mysql admin db: %v", err)
	}
	t.Cleanup(func() {
		_ = adminDB.Close()
	})

	if _, err := adminDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
		t.Fatalf("drop migration test database: %v", err)
	}
	if _, err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create migration test database: %v", err)
	}

	testCfg := *parsed
	testCfg.DBName = databaseName
	testCfg.MultiStatements = true
	testCfg.ParseTime = true
	if testCfg.Loc == nil {
		testCfg.Loc = time.Local
	}

	return testCfg.FormatDSN()
}
