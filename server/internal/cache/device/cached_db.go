package device

import (
	"database/sql"
	"errors"

	"github.com/lucap9056/corvauth/database"
	"github.com/lucap9056/corvauth/jwt"
	"golang.org/x/sync/singleflight"
)

type CachedDB struct {
	*database.Database
	cache       SecretCache
	secretLoads singleflight.Group
}

func NewCachedDB(db *database.Database, cache SecretCache) *CachedDB {
	return &CachedDB{Database: db, cache: cache}
}

func (c *CachedDB) GetDeviceSecret(deviceID string) (string, int, error) {
	if secret, ok := c.cache.GetSecret(deviceID); ok {
		return secret.Value, secret.Generation, nil
	}

	v, err, _ := c.secretLoads.Do(deviceID, func() (any, error) {
		value, generation, err := c.Database.GetDeviceSecret(deviceID)
		if errors.Is(err, sql.ErrNoRows) {
			return Secret{}, jwt.ErrDeviceNotFound
		}
		if err != nil {
			return Secret{}, err
		}
		secret := Secret{Value: value, Generation: generation}
		c.cache.SetSecret(deviceID, secret)
		return secret, nil
	})
	if err != nil {
		return "", 0, err
	}
	secret := v.(Secret)
	return secret.Value, secret.Generation, nil
}

func (c *CachedDB) UpdateDeviceSecret(deviceID string) (int, error) {
	generation, err := c.Database.UpdateDeviceSecret(deviceID)
	if err != nil {
		return 0, err
	}

	secret, ok := c.cache.GetSecret(deviceID)
	if !ok {
		value, current, err := c.Database.GetDeviceSecret(deviceID)
		if err != nil {
			return generation, nil
		}
		secret = Secret{Value: value, Generation: current}
	}
	secret.Generation = max(secret.Generation, generation)
	c.cache.SetSecret(deviceID, secret)
	return generation, nil
}

func (c *CachedDB) DeleteDevice(userEmail, deviceID string) error {
	if err := c.Database.DeleteDevice(userEmail, deviceID); err != nil {
		return err
	}
	c.cache.DeleteSecret(deviceID)
	return nil
}

func (c *CachedDB) DeleteAllDevices(userEmail string) error {
	ids, err := c.Database.DeleteAllDevices(userEmail)
	if err != nil {
		return err
	}

	for _, deviceID := range ids {
		c.cache.DeleteSecret(deviceID)
	}
	return nil
}
