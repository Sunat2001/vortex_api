package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/voronka/backend/internal/shared/config"
	"github.com/voronka/backend/internal/shared/database"
	"github.com/voronka/backend/internal/shared/migration"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Simple logger for migrations
	logger, _ := zap.NewDevelopment()
	if cfg.App.Env == "production" {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()

	// Connect only to PostgreSQL (no Redis needed for migrations)
	pgPool, err := database.NewPostgresPool(ctx, &cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to PostgreSQL", zap.Error(err))
	}
	defer database.Close(pgPool, logger)

	// Get migrations directory path
	migrationsPath, err := filepath.Abs("migrations")
	if err != nil {
		logger.Fatal("failed to get migrations path", zap.Error(err))
	}

	// Check if migrations directory exists
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		logger.Fatal("migrations directory not found", zap.String("path", migrationsPath))
	}

	// Create migrator
	migrator, err := migration.NewMigrator(pgPool, migrationsPath, logger)
	if err != nil {
		logger.Fatal("failed to create migrator", zap.Error(err))
	}
	defer migrator.Close()

	command := os.Args[1]

	switch command {
	case "up":
		if err := migrator.Up(ctx); err != nil {
			logger.Fatal("migration up failed", zap.Error(err))
		}
		fmt.Println("Migrations applied successfully!")

	case "down":
		if err := migrator.Down(ctx); err != nil {
			logger.Fatal("migration down failed", zap.Error(err))
		}
		fmt.Println("Migration rolled back successfully!")

	case "steps":
		if len(os.Args) < 3 {
			logger.Fatal("steps command requires a number argument")
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			logger.Fatal("invalid steps argument", zap.Error(err))
		}
		if err := migrator.Steps(ctx, n); err != nil {
			logger.Fatal("migration steps failed", zap.Error(err))
		}
		fmt.Printf("Applied %d migration steps\n", n)

	case "version":
		version, dirty, err := migrator.Version()
		if err != nil {
			logger.Fatal("failed to get version", zap.Error(err))
		}
		if version == 0 {
			fmt.Println("No migrations applied yet")
		} else {
			fmt.Printf("Current version: %d (dirty: %v)\n", version, dirty)
		}

	case "force":
		if len(os.Args) < 3 {
			logger.Fatal("force command requires a version argument")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			logger.Fatal("invalid version argument", zap.Error(err))
		}
		if err := migrator.Force(v); err != nil {
			logger.Fatal("force version failed", zap.Error(err))
		}
		fmt.Printf("Forced version to %d\n", v)

	case "create":
		if len(os.Args) < 3 {
			logger.Fatal("create command requires a migration name")
		}
		name := os.Args[2]
		if err := createMigration(migrationsPath, name); err != nil {
			logger.Fatal("failed to create migration", zap.Error(err))
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func createMigration(migrationsPath, name string) error {
	// Find next version number
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	maxVersion := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		if len(fileName) < 6 {
			continue
		}
		versionStr := fileName[:6]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}
		if version > maxVersion {
			maxVersion = version
		}
	}

	nextVersion := maxVersion + 1
	timestamp := time.Now().Format("20060102")

	// Sanitize name
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ToLower(name)

	upFile := filepath.Join(migrationsPath, fmt.Sprintf("%06d_%s.up.sql", nextVersion, name))
	downFile := filepath.Join(migrationsPath, fmt.Sprintf("%06d_%s.down.sql", nextVersion, name))

	// Create up migration
	upContent := fmt.Sprintf("-- Migration: %s\n-- Created: %s\n\n-- Add your UP migration SQL here\n", name, timestamp)
	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}

	// Create down migration
	downContent := fmt.Sprintf("-- Rollback: %s\n-- Created: %s\n\n-- Add your DOWN migration SQL here\n", name, timestamp)
	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	fmt.Printf("Created migrations:\n")
	fmt.Printf("  %s\n", upFile)
	fmt.Printf("  %s\n", downFile)

	return nil
}

func printUsage() {
	fmt.Println("Vortex Database Migration Tool")
	fmt.Println("")
	fmt.Println("Usage: go run cmd/migrate/main.go [command]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  up              Run all pending migrations")
	fmt.Println("  down            Rollback last migration")
	fmt.Println("  steps N         Run N migrations (positive=up, negative=down)")
	fmt.Println("  version         Show current migration version")
	fmt.Println("  force VERSION   Force set migration version (use with caution)")
	fmt.Println("  create NAME     Create new migration files")
	fmt.Println("")
	fmt.Println("Environment variables:")
	fmt.Println("  DATABASE_HOST      PostgreSQL host (default: localhost)")
	fmt.Println("  DATABASE_PORT      PostgreSQL port (default: 5432)")
	fmt.Println("  DATABASE_USER      PostgreSQL user (default: postgres)")
	fmt.Println("  DATABASE_PASSWORD  PostgreSQL password (default: postgres)")
	fmt.Println("  DATABASE_DATABASE  Database name (default: voronka)")
}