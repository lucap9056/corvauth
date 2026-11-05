package database

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func newMockDB(t *testing.T) (*Database, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		_ = db.Close()
		cancel()
	})
	return &Database{db: db, ctx: ctx, cancel: cancel}, mock
}

func TestDeleteAllDevices_RowError(t *testing.T) {
	d, mock := newMockDB(t)

	mock.ExpectQuery(`DELETE FROM auth_user_devices`).
		WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"device_id"}).
			AddRow("dev-1").
			AddRow("dev-2").
			RowError(1, errors.New("row failed")))

	if _, err := d.DeleteAllDevices("user@example.com"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
