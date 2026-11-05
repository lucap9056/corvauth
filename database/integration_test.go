package database_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lucap9056/corvauth/database"
)

type testEnv struct {
	schema string
	dsn    string
	db     *sql.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	schemaName := "test_" + hex.EncodeToString(suffix)

	dsn, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	query := dsn.Query()
	query.Set("search_path", schemaName)
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	env := &testEnv{schema: schemaName, dsn: dsn.String(), db: db}
	env.exec(t, "CREATE SCHEMA "+schemaName)
	t.Cleanup(func() { db.Exec("DROP SCHEMA " + schemaName + " CASCADE") })
	return env
}

func (e *testEnv) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func (e *testEnv) createUsers(t *testing.T, emails ...string) {
	t.Helper()
	e.exec(t, "CREATE TABLE users (email TEXT PRIMARY KEY)")
	for _, email := range emails {
		e.exec(t, "INSERT INTO users (email) VALUES ($1)", email)
	}
}

func (e *testEnv) open(t *testing.T, opts ...database.Option) (*database.Database, error) {
	t.Helper()
	defaults := []database.Option{
		database.WithAutoCreateSchema(true),
		database.WithCleanupInterval(0),
	}
	d, err := database.NewDatabase(e.dsn, append(defaults, opts...)...)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { d.Close() })
	return d, nil
}

func (e *testEnv) mustOpen(t *testing.T, opts ...database.Option) *database.Database {
	t.Helper()
	d, err := e.open(t, opts...)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	return d
}

func newTestDatabase(t *testing.T, emails ...string) (*testEnv, *database.Database) {
	t.Helper()
	env := newTestEnv(t)
	env.createUsers(t, emails...)
	return env, env.mustOpen(t)
}

func mustSaveDevice(t *testing.T, d *database.Database, email string) string {
	t.Helper()
	deviceID, err := d.SaveDeviceSecret(email, "phone", "secret")
	if err != nil {
		t.Fatalf("SaveDeviceSecret(%q): %v", email, err)
	}
	return deviceID
}

func deviceExists(t *testing.T, d *database.Database, deviceID string) bool {
	t.Helper()
	_, _, err := d.GetDeviceSecret(deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatalf("GetDeviceSecret(%q): %v", deviceID, err)
	}
	return true
}

func TestNewDatabase_ConcurrentAutoCreateSchema(t *testing.T) {
	env := newTestEnv(t)
	env.createUsers(t)

	const instances = 5
	errs := make([]error, instances)
	var wg sync.WaitGroup
	for i := range instances {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = env.open(t)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("instance %d: %v", i, err)
		}
	}
}

func TestNewDatabase_EmailTypeMismatch(t *testing.T) {
	env := newTestEnv(t)
	env.exec(t, "CREATE TABLE users (email INTEGER PRIMARY KEY)")

	if _, err := env.open(t); err == nil {
		t.Fatal("expected error for incompatible email column type, got nil")
	}
}

func TestNewDatabase_CustomUserEmailReference(t *testing.T) {
	env := newTestEnv(t)
	env.exec(t, "CREATE TABLE members (mail VARCHAR(255) NOT NULL UNIQUE)")
	env.exec(t, "INSERT INTO members (mail) VALUES ($1)", "member@example.com")

	userRef, err := database.WithUserEmailReference(env.schema + ".members(mail):varchar(255)")
	if err != nil {
		t.Fatal(err)
	}
	d := env.mustOpen(t, userRef)

	mustSaveDevice(t, d, "member@example.com")
	if _, err := d.SaveDeviceSecret("nobody@example.com", "phone", "secret"); !errors.Is(err, database.ErrUserNotFound) {
		t.Errorf("err: got %v, want ErrUserNotFound", err)
	}
}

func TestSaveDeviceSecret_UnknownUser(t *testing.T) {
	_, d := newTestDatabase(t)

	if _, err := d.SaveDeviceSecret("nobody@example.com", "phone", "secret"); !errors.Is(err, database.ErrUserNotFound) {
		t.Fatalf("err: got %v, want ErrUserNotFound", err)
	}
}

func TestDeviceSecret_GenerationLifecycle(t *testing.T) {
	_, d := newTestDatabase(t, "user@example.com")
	deviceID := mustSaveDevice(t, d, "user@example.com")

	for want := 2; want <= 3; want++ {
		generation, err := d.UpdateDeviceSecret(deviceID)
		if err != nil {
			t.Fatalf("UpdateDeviceSecret: %v", err)
		}
		if generation != want {
			t.Errorf("UpdateDeviceSecret generation: got %d, want %d", generation, want)
		}
	}

	secret, generation, err := d.GetDeviceSecret(deviceID)
	if err != nil {
		t.Fatalf("GetDeviceSecret: %v", err)
	}
	if secret != "secret" || generation != 3 {
		t.Errorf("GetDeviceSecret: got (%q, %d), want (%q, %d)", secret, generation, "secret", 3)
	}
}

func TestDeviceSecret_UnknownDevice(t *testing.T) {
	_, d := newTestDatabase(t)
	const unknownID = "00000000-0000-0000-0000-000000000000"

	if _, _, err := d.GetDeviceSecret(unknownID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetDeviceSecret err: got %v, want sql.ErrNoRows", err)
	}
	if _, err := d.UpdateDeviceSecret(unknownID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateDeviceSecret err: got %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteDevice_RequiresOwner(t *testing.T) {
	_, d := newTestDatabase(t, "owner@example.com", "other@example.com")
	deviceID := mustSaveDevice(t, d, "owner@example.com")

	if err := d.DeleteDevice("other@example.com", deviceID); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}
	if !deviceExists(t, d, deviceID) {
		t.Fatal("device deleted by a different user")
	}

	if err := d.DeleteDevice("owner@example.com", deviceID); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}
	if deviceExists(t, d, deviceID) {
		t.Error("device still exists after owner deleted it")
	}
}

func TestDeleteAllDevices_OnlyTargetUser(t *testing.T) {
	_, d := newTestDatabase(t, "target@example.com", "other@example.com")
	want := []string{
		mustSaveDevice(t, d, "target@example.com"),
		mustSaveDevice(t, d, "target@example.com"),
	}
	otherDevice := mustSaveDevice(t, d, "other@example.com")

	got, err := d.DeleteAllDevices("target@example.com")
	if err != nil {
		t.Fatalf("DeleteAllDevices: %v", err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("deleted IDs: got %v, want %v", got, want)
	}
	if !deviceExists(t, d, otherDevice) {
		t.Error("device of another user was deleted")
	}
}

func TestListDevices_NewestFirst(t *testing.T) {
	env, d := newTestDatabase(t, "target@example.com", "other@example.com")
	older := mustSaveDevice(t, d, "target@example.com")
	newer := mustSaveDevice(t, d, "target@example.com")
	mustSaveDevice(t, d, "other@example.com")
	env.exec(t, "UPDATE auth_user_devices SET updated_at = updated_at - INTERVAL '1 hour' WHERE device_id = $1", older)

	got, err := d.ListDevices("target@example.com")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(got) != 2 || got[0].DeviceID != newer || got[1].DeviceID != older {
		t.Errorf("devices: got %+v, want [%s %s]", got, newer, older)
	}
}

func TestUserDeletion_CascadesToDevices(t *testing.T) {
	env, d := newTestDatabase(t, "user@example.com")
	deviceID := mustSaveDevice(t, d, "user@example.com")

	env.exec(t, "DELETE FROM users WHERE email = $1", "user@example.com")

	if deviceExists(t, d, deviceID) {
		t.Error("device still exists after its user was deleted")
	}
}

func TestUserEmailChange_CascadesToDevices(t *testing.T) {
	env, d := newTestDatabase(t, "old@example.com")
	deviceID := mustSaveDevice(t, d, "old@example.com")

	env.exec(t, "UPDATE users SET email = $1 WHERE email = $2", "new@example.com", "old@example.com")

	deleted, err := d.DeleteAllDevices("new@example.com")
	if err != nil {
		t.Fatalf("DeleteAllDevices: %v", err)
	}
	if !slices.Equal(deleted, []string{deviceID}) {
		t.Errorf("devices under new email: got %v, want [%s]", deleted, deviceID)
	}
}

func TestCleanupWorker_RemovesDevicesNotUpdatedFor7Days(t *testing.T) {
	env := newTestEnv(t)
	env.createUsers(t, "user@example.com")
	d := env.mustOpen(t, database.WithCleanupInterval(20*time.Millisecond))

	staleDevice := mustSaveDevice(t, d, "user@example.com")
	recentDevice := mustSaveDevice(t, d, "user@example.com")
	env.exec(t, "UPDATE auth_user_devices SET updated_at = NOW() - INTERVAL '8 days' WHERE device_id = $1", staleDevice)
	env.exec(t, "UPDATE auth_user_devices SET updated_at = NOW() - INTERVAL '6 days' WHERE device_id = $1", recentDevice)

	deadline := time.Now().Add(5 * time.Second)
	for deviceExists(t, d, staleDevice) {
		if time.Now().After(deadline) {
			t.Fatal("stale device was not cleaned up")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !deviceExists(t, d, recentDevice) {
		t.Error("device updated within 7 days was cleaned up")
	}
}

func TestNew_SharesCallerDB(t *testing.T) {
	env := newTestEnv(t)
	env.createUsers(t, "a@example.com")

	d, err := database.New(env.db,
		database.WithAutoCreateSchema(true),
		database.WithCleanupInterval(0),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mustSaveDevice(t, d, "a@example.com")

	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := env.db.Ping(); err != nil {
		t.Fatalf("caller db should stay open after Close: %v", err)
	}
}
