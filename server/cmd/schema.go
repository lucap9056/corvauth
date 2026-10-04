package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/server/internal/config"
	"github.com/lucap9056/corvauth/server/internal/usersdb"
)

var (
	errUsage               = errors.New("usage: corvauth schema <apply|print>")
	errDatabaseURLRequired = fmt.Errorf("%s is required", config.EnvDatabaseURL)
)

func runCommand(args []string, stdout io.Writer) error {
	if len(args) != 2 || args[0] != "schema" {
		return errUsage
	}
	switch args[1] {
	case "apply":
		return applySchema()
	case "print":
		return printSchema(stdout)
	}
	return errUsage
}

func printSchema(w io.Writer) error {
	ddl, err := usersdb.GenerateSchema(os.Getenv(config.EnvDBUserEmailReference))
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	_, err = io.WriteString(w, ddl)
	return err
}

func applySchema() error {
	cfg, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if cfg == nil {
		return errDatabaseURLRequired
	}

	sqlDB, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer sqlDB.Close()

	opts := append(usersOptions(cfg),
		usersdb.WithAutoCreateSchema(true),
		usersdb.WithDatabaseOptions(database.WithCleanupInterval(0)),
	)
	store, err := usersdb.New(sqlDB, opts...)
	if err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}
	store.Close()

	log.Println("Schema applied")
	return nil
}
