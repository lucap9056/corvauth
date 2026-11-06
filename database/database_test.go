package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lucap9056/corvauth/database/schema"
)

const unreachableDSN = "postgres://user:pass@127.0.0.1:1/db?connect_timeout=1"

func TestCreateSchema_RollbackOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).
		WithArgs(schemaAdvisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS auth_user_devices`).
		WillReturnError(errors.New("relation users does not exist"))
	mock.ExpectRollback()

	if err := createSchema(db, schema.DefaultParams()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreateSchema_InvalidParams(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := createSchema(db, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestNew_RejectsNonPgxDriver(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := New(db); !errors.Is(err, ErrUnsupportedDriver) {
		t.Fatalf("err = %v; want ErrUnsupportedDriver", err)
	}
}

func TestNew_AcceptsPgxPool(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), unreachableDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	if _, err := New(db); err == nil || errors.Is(err, ErrUnsupportedDriver) {
		t.Fatalf("err = %v; want a connection error", err)
	}
}

func TestNew_FailureKeepsCallerDBOpen(t *testing.T) {
	db, err := sql.Open("pgx", unreachableDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := New(db); err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if err := db.Ping(); err == nil || err.Error() == "sql: database is closed" {
		t.Fatalf("caller db should stay open, Ping err = %v", err)
	}
}

func TestCleanupWorker_ReportsErrorToHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cleanupErr := errors.New("connection reset")
	mock.ExpectExec("DELETE FROM auth_user_devices").WillReturnError(cleanupErr)

	reported := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &Database{db: db, onCleanupError: func(err error) { reported <- err }, ctx: ctx}
	go d.startCleanupWorker(time.Millisecond)

	if got := <-reported; !errors.Is(got, cleanupErr) {
		t.Errorf("got %v, want %v", got, cleanupErr)
	}
}
