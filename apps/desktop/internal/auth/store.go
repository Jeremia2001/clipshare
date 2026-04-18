// Package auth handles persistence of the desktop client's credentials.
// Device tokens are stored in the OS keyring (Windows Credential Manager via
// DPAPI, macOS Keychain, libsecret on Linux). The server URL and account
// username are persisted to the config file so the UI can offer them on
// launch without touching the keyring until a request actually needs a token.
package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "clipshare"
	tokenKey       = "device_token"
)

// ErrNotFound matches go-keyring.ErrNotFound.
var ErrNotFound = keyring.ErrNotFound

// SaveDeviceToken persists the device token in the OS keyring, keyed by the
// server URL so users running multiple instances stay separated.
func SaveDeviceToken(serverURL, token string) error {
	if serverURL == "" {
		return errors.New("server URL is required")
	}
	return keyring.Set(keyringService, keyAccount(serverURL), token)
}

// LoadDeviceToken returns the device token for the given server, or empty string
// if none is stored. A missing entry is not an error.
func LoadDeviceToken(serverURL string) (string, error) {
	if serverURL == "" {
		return "", nil
	}
	tok, err := keyring.Get(keyringService, keyAccount(serverURL))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return tok, nil
}

func DeleteDeviceToken(serverURL string) error {
	if serverURL == "" {
		return nil
	}
	err := keyring.Delete(keyringService, keyAccount(serverURL))
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

// keyAccount ensures one stored credential per server URL.
func keyAccount(serverURL string) string {
	return "device:" + serverURL
}
