package db

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SQLiteConfig struct {
	BusyTimeoutMs int
	WAL           bool
	ForeignKeys   bool
}

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type Config struct {
	Driver      string
	DSN         string
	Pool        PoolConfig
	SQLite      SQLiteConfig
	AutoMigrate bool
}

func DefaultConfig() Config {
	return Config{
		Driver: "sqlite",
		DSN:    "",
		Pool: PoolConfig{
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: 0,
		},
		SQLite: SQLiteConfig{
			BusyTimeoutMs: 5000,
			WAL:           true,
			ForeignKeys:   true,
		},
		AutoMigrate: true,
	}
}

func ResolveSQLiteDSN(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn != "" {
		return dsn, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	homeDir := filepath.Join(home, ".morph")
	homeDB := filepath.Join(homeDir, "mistermorph.sqlite")
	localDB := filepath.Clean("./mistermorph.sqlite")

	// Precedence:
	// 1) existing $HOME/.morph/mistermorph.sqlite
	if _, err := os.Stat(homeDB); err == nil {
		return homeDB, nil
	}
	// 2) existing ./mistermorph.sqlite
	if _, err := os.Stat(localDB); err == nil {
		return localDB, nil
	}
	// 3) create + use $HOME/.morph/mistermorph.sqlite (ensure dir exists)
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		return "", err
	}
	return homeDB, nil
}
