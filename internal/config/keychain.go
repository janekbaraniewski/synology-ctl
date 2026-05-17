package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "synoctl"

// SavePassword stores a password in the OS keychain keyed by host+user.
func SavePassword(host, user, password string) error {
	return keyring.Set(serviceName, key(host, user), password)
}

// LoadPassword fetches the stored password, returning "" with no error when
// nothing is stored (so the caller can prompt interactively).
func LoadPassword(host, user string) (string, error) {
	pw, err := keyring.Get(serviceName, key(host, user))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return pw, nil
}

// DeletePassword removes a stored password. It's not an error if it didn't exist.
func DeletePassword(host, user string) error {
	err := keyring.Delete(serviceName, key(host, user))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func key(host, user string) string {
	return fmt.Sprintf("%s@%s", user, host)
}
