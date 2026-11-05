package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgForeignKeyViolation   = "23503"
	userDevicesFKConstraint = "fk_auth_user_devices"
)

var ErrUserNotFound = errors.New("user not found")

type UserDevice struct {
	UserEmail  string `json:"user_email"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Secret     string `json:"secret,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

func (d *Database) SaveDeviceSecret(userEmail, deviceName, secret string) (string, error) {
	query := `
	INSERT INTO auth_user_devices (device_name, user_email, secret, updated_at)
	VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
	RETURNING device_id;
	`
	var deviceID string
	err := d.db.QueryRow(query, deviceName, userEmail, secret).Scan(&deviceID)
	if err != nil {

		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == pgForeignKeyViolation &&
			pgErr.ConstraintName == userDevicesFKConstraint {

			return "", ErrUserNotFound
		}

		return "", err
	}
	return deviceID, nil
}

func (d *Database) UpdateDeviceSecret(deviceID string) (int, error) {
	query := `
	UPDATE auth_user_devices
	SET generation = generation + 1, updated_at = CURRENT_TIMESTAMP
	WHERE device_id = $1
	RETURNING generation;
	`
	var generation int
	err := d.db.QueryRow(query, deviceID).Scan(&generation)
	if err != nil {
		return 0, err
	}
	return generation, nil
}

func (d *Database) GetDeviceSecret(deviceID string) (string, int, error) {
	var secret string
	var generation int
	err := d.db.QueryRow("SELECT secret, generation FROM auth_user_devices WHERE device_id = $1", deviceID).Scan(&secret, &generation)
	if err != nil {
		return "", 0, err
	}
	return secret, generation, nil
}

func (d *Database) DeleteDevice(userEmail, deviceID string) error {
	query := `DELETE FROM auth_user_devices WHERE user_email = $1 AND device_id = $2`
	_, err := d.db.Exec(query, userEmail, deviceID)
	return err
}

func (d *Database) DeleteAllDevices(userEmail string) ([]string, error) {
	query := `DELETE FROM auth_user_devices WHERE user_email = $1 RETURNING device_id`
	rows, err := d.db.Query(query, userEmail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deviceIDs []string
	for rows.Next() {
		var deviceID string
		if err := rows.Scan(&deviceID); err != nil {
			return nil, err
		}
		deviceIDs = append(deviceIDs, deviceID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return deviceIDs, nil
}
