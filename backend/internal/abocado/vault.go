package abocado

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type OAuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenURL     string    `json:"token_url"`
	ClientID     string    `json:"client_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type TokenVault struct{ Path string }

func (v TokenVault) Save(tokens OAuthTokens) error {
	b, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	sealed, err := protectCurrentUser(b)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(v.Path), 0700); err != nil {
		return err
	}
	return os.WriteFile(v.Path, sealed, 0600)
}

func (v TokenVault) Load() (OAuthTokens, error) {
	var tokens OAuthTokens
	sealed, err := os.ReadFile(v.Path)
	if err != nil {
		return tokens, err
	}
	b, err := unprotectCurrentUser(sealed)
	if err != nil {
		return tokens, err
	}
	err = json.Unmarshal(b, &tokens)
	if tokens.AccessToken == "" {
		return tokens, errors.New("Abocado OAuth token is missing")
	}
	return tokens, err
}

func (v TokenVault) Clear() error {
	err := os.Remove(v.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

