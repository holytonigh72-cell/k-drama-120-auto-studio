//go:build windows

package abocado

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTokenVaultUsesWindowsDPAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "abocado.oauth")
	vault := TokenVault{Path: path}
	input := OAuthTokens{AccessToken: "test-only-fake-token", RefreshToken: "test-only-fake-refresh", ClientID: "test-client"}
	if err := vault.Save(input); err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(input.AccessToken)) || bytes.Contains(sealed, []byte(input.RefreshToken)) {
		t.Fatal("OAuth secrets were written as plaintext")
	}
	got, err := vault.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != input.AccessToken || got.RefreshToken != input.RefreshToken {
		t.Fatalf("DPAPI round-trip mismatch: %+v", got)
	}
}

