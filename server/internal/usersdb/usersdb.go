package usersdb

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/jackc/pgx/v5"
	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/database/schema"
)

const schemaAdvisoryLockKey int64 = 0x6175746875736572

//go:embed schema.sql
var schemaSQL string

var schemaTemplate = template.Must(template.New("schema.sql").Parse(schemaSQL))

func generateUsersSchema() (string, error) {
	var s strings.Builder
	if err := schemaTemplate.Execute(&s, schema.DefaultParams()); err != nil {
		return "", err
	}
	return s.String(), nil
}

func GenerateSchema(userEmailReference string) (string, error) {
	if userEmailReference != "" {
		params, err := schema.ParseUserEmailReference(userEmailReference)
		if err != nil {
			return "", err
		}
		return schema.Generate(params)
	}

	usersSchema, err := generateUsersSchema()
	if err != nil {
		return "", err
	}
	devicesSchema, err := schema.Generate(schema.DefaultParams())
	if err != nil {
		return "", err
	}
	return usersSchema + "\n\n" + devicesSchema, nil
}

func createSchema(db *sql.DB) error {
	usersSchema, err := generateUsersSchema()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("SELECT pg_advisory_xact_lock($1)", schemaAdvisoryLockKey); err != nil {
		return err
	}
	if _, err := tx.Exec(usersSchema); err != nil {
		return err
	}
	return tx.Commit()
}

var ErrExternalUsers = errors.New("users table is managed externally")

type Store struct {
	*database.Database
	db                  *sql.DB
	external            bool
	selectUsernameQuery string
}

func New(db *sql.DB, opts ...Option) (*Store, error) {
	cfg := newOptions(opts)

	store, err := newStore(db, cfg)
	if err != nil {
		return nil, err
	}

	databaseOptions := append(cfg.databaseOptions, database.WithAutoCreateSchema(cfg.autoCreateSchema))
	if cfg.external != nil {
		userEmailReference, err := database.WithUserEmailReference(cfg.external.userEmailReference)
		if err != nil {
			return nil, err
		}
		databaseOptions = append(databaseOptions, userEmailReference)
	}

	store.Database, err = database.New(db, databaseOptions...)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func newStore(db *sql.DB, cfg *options) (*Store, error) {
	if cfg.external != nil {
		return newExternal(db, cfg.external)
	}

	if cfg.autoCreateSchema {
		if err := createSchema(db); err != nil {
			return nil, err
		}
	} else if err := probe(db, "SELECT user_id, username, email FROM users LIMIT 0"); err != nil {
		return nil, err
	}

	return &Store{db: db, selectUsernameQuery: "SELECT username FROM users WHERE email = $1"}, nil
}

func newExternal(db *sql.DB, cfg *externalOptions) (*Store, error) {
	store := &Store{db: db, external: true}
	if cfg.usernameColumn == "" {
		return store, nil
	}

	params, err := schema.ParseUserEmailReference(cfg.userEmailReference)
	if err != nil {
		return nil, err
	}
	column := pgx.Identifier{cfg.usernameColumn}.Sanitize()

	if err := probe(db, fmt.Sprintf("SELECT %s FROM %s LIMIT 0", column, params.UsersTable)); err != nil {
		return nil, err
	}

	store.selectUsernameQuery = fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", column, params.UsersTable, params.UsersEmailColumn)
	return store, nil
}

func probe(db *sql.DB, query string) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	return rows.Close()
}

type User struct {
	ID       string
	Username string
	Email    string
}

func (s *Store) External() bool {
	return s.external
}

func (s *Store) CreateUser(username, email string) (*User, error) {
	if s.external {
		return nil, ErrExternalUsers
	}
	var user User
	err := s.db.QueryRow(
		"INSERT INTO users (username, email) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING user_id, username, email",
		username, email,
	).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUser(email string) (*User, error) {
	if s.external {
		return nil, ErrExternalUsers
	}
	var user User
	err := s.db.QueryRow(
		"SELECT user_id, username, email FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Username, &user.Email)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUsername(email string) (string, error) {
	if s.selectUsernameQuery == "" {
		return "", nil
	}

	var username sql.NullString
	err := s.db.QueryRow(s.selectUsernameQuery, email).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", database.ErrUserNotFound
	}
	if err != nil {
		return "", err
	}
	return username.String, nil
}

func (s *Store) DeleteUser(email string) error {
	if s.external {
		return ErrExternalUsers
	}
	_, err := s.db.Exec(
		"DELETE FROM users WHERE email = $1",
		email,
	)
	return err
}
