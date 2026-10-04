package usersdb

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/database/schema"
)

const (
	externalReference    = "auth.members(mail):citext"
	externalProbeQuery   = `SELECT "display_name" FROM "auth"."members" LIMIT 0`
	externalSelectQuery  = `SELECT "display_name" FROM "auth"."members" WHERE "mail" = $1`
	managedSelectQuery   = `SELECT username FROM users WHERE email = $1`
	externalUsernameName = "display_name"
)

func newMock(t *testing.T) (sqlmock.Sqlmock, func(opts ...Option) (*Store, error)) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})
	return mock, func(opts ...Option) (*Store, error) { return newStore(db, newOptions(opts)) }
}

func expectQuery(mock sqlmock.Sqlmock, query string) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(regexp.QuoteMeta(query))
}

func newExternalStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	mock, newStore := newMock(t)
	expectQuery(mock, externalProbeQuery).WillReturnRows(sqlmock.NewRows([]string{externalUsernameName}))

	store, err := newStore(WithExternal(externalReference, externalUsernameName))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store, mock
}

func TestNew_ManagedCreatesSchema(t *testing.T) {
	mock, newStore := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(schemaAdvisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "users"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	expectQuery(mock, managedSelectQuery).WithArgs("a@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("Alice"))

	store, err := newStore(WithAutoCreateSchema(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, err := store.GetUsername("a@example.com"); err != nil || got != "Alice" {
		t.Fatalf("GetUsername = %q, %v; want Alice, nil", got, err)
	}
}

func TestNew_ExternalSkipsSchema(t *testing.T) {
	store, mock := newExternalStore(t)
	expectQuery(mock, externalSelectQuery).WithArgs("a@example.com").
		WillReturnRows(sqlmock.NewRows([]string{externalUsernameName}).AddRow("Alice"))

	if got, err := store.GetUsername("a@example.com"); err != nil || got != "Alice" {
		t.Fatalf("GetUsername = %q, %v; want Alice, nil", got, err)
	}
}

func TestNew_ExternalInvalidReference(t *testing.T) {
	_, newStore := newMock(t)

	if _, err := newStore(WithExternal("members", externalUsernameName)); !errors.Is(err, schema.ErrInvalidUserEmailReference) {
		t.Fatalf("err = %v; want ErrInvalidUserEmailReference", err)
	}
}

func TestNew_ExternalUnreadableUsernameColumn(t *testing.T) {
	mock, newStore := newMock(t)
	expectQuery(mock, externalProbeQuery).WillReturnError(errors.New(`column "display_name" does not exist`))

	if _, err := newStore(WithExternal(externalReference, externalUsernameName)); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetUsername_ExternalWithoutColumn(t *testing.T) {
	_, newStore := newMock(t)

	store, err := newStore(WithExternal(externalReference, ""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, err := store.GetUsername("a@example.com"); err != nil || got != "" {
		t.Fatalf("GetUsername = %q, %v; want empty, nil", got, err)
	}
}

func TestGetUsername_NullIsEmpty(t *testing.T) {
	store, mock := newExternalStore(t)
	expectQuery(mock, externalSelectQuery).WithArgs("a@example.com").
		WillReturnRows(sqlmock.NewRows([]string{externalUsernameName}).AddRow(nil))

	if got, err := store.GetUsername("a@example.com"); err != nil || got != "" {
		t.Fatalf("GetUsername = %q, %v; want empty, nil", got, err)
	}
}

func TestGetUsername_UnknownUser(t *testing.T) {
	store, mock := newExternalStore(t)
	expectQuery(mock, externalSelectQuery).WithArgs("missing@example.com").
		WillReturnRows(sqlmock.NewRows([]string{externalUsernameName}))

	if _, err := store.GetUsername("missing@example.com"); !errors.Is(err, database.ErrUserNotFound) {
		t.Fatalf("err = %v; want ErrUserNotFound", err)
	}
}

func TestExternalStore_RejectsUserManagement(t *testing.T) {
	store, _ := newExternalStore(t)

	if _, err := store.CreateUser("Alice", "a@example.com"); !errors.Is(err, ErrExternalUsers) {
		t.Errorf("CreateUser err = %v; want ErrExternalUsers", err)
	}
	if _, err := store.GetUser("a@example.com"); !errors.Is(err, ErrExternalUsers) {
		t.Errorf("GetUser err = %v; want ErrExternalUsers", err)
	}
	if err := store.DeleteUser("a@example.com"); !errors.Is(err, ErrExternalUsers) {
		t.Errorf("DeleteUser err = %v; want ErrExternalUsers", err)
	}
}

func TestNew_PropagatesDatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(schemaAdvisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "users"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if _, err := New(db, WithAutoCreateSchema(true)); !errors.Is(err, database.ErrUnsupportedDriver) {
		t.Fatalf("err = %v; want ErrUnsupportedDriver", err)
	}
}

func TestNew_ExternalInvalidReferenceWithoutColumn(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := New(db, WithExternal("members", "")); !errors.Is(err, schema.ErrInvalidUserEmailReference) {
		t.Fatalf("err = %v; want ErrInvalidUserEmailReference", err)
	}
}

func TestCreateUser_ReturnsExistingUserOnConflict(t *testing.T) {
	mock, newStore := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(schemaAdvisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "users"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	expectQuery(mock, "INSERT INTO users (username, email) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING user_id, username, email").
		WithArgs("New Name", "a@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email"}).AddRow("id-1", "Existing Name", "a@example.com"))

	store, err := newStore(WithAutoCreateSchema(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	user, err := store.CreateUser("New Name", "a@example.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Username != "Existing Name" {
		t.Errorf("Username = %q; want existing user's name", user.Username)
	}
}

func TestNew_ManagedWithoutAutoCreateChecksTable(t *testing.T) {
	mock, newStore := newMock(t)
	expectQuery(mock, "SELECT user_id, username, email FROM users LIMIT 0").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email"}))

	if _, err := newStore(); err != nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNew_ManagedWithoutAutoCreateMissingTable(t *testing.T) {
	mock, newStore := newMock(t)
	expectQuery(mock, "SELECT user_id, username, email FROM users LIMIT 0").
		WillReturnError(errors.New(`relation "users" does not exist`))

	if _, err := newStore(); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGenerateSchema(t *testing.T) {
	for reference, wantUsers := range map[string]bool{"": true, externalReference: false} {
		ddl, err := GenerateSchema(reference)
		if err != nil {
			t.Fatalf("GenerateSchema(%q): %v", reference, err)
		}
		if got := strings.Contains(ddl, `CREATE TABLE IF NOT EXISTS "users"`); got != wantUsers {
			t.Errorf("GenerateSchema(%q) creates users = %v; want %v", reference, got, wantUsers)
		}
		if !strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS auth_user_devices") {
			t.Errorf("GenerateSchema(%q) missing auth_user_devices", reference)
		}
	}
}
