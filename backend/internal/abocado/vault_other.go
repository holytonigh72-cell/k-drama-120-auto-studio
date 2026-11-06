//go:build !windows

package abocado

import "errors"

func protectCurrentUser([]byte) ([]byte, error) {
	return nil, errors.New("OAuth tokens can only be stored on Windows with DPAPI")
}
func unprotectCurrentUser([]byte) ([]byte, error) {
	return nil, errors.New("OAuth tokens can only be read on Windows with DPAPI")
}

