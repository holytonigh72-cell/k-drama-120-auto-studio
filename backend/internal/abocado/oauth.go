package abocado

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OAuthFlow struct {
	URL  string
	Done <-chan error
	stop func()
}

func (f *OAuthFlow) Close() {
	if f != nil && f.stop != nil {
		f.stop()
	}
}

type protectedResourceMetadata struct {
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}
type authorizationMetadata struct {
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
}
type tokenReply struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// BeginOAuth follows the MCP OAuth protected-resource metadata flow and uses
// PKCE. It keeps all tokens in the Windows DPAPI vault; the browser only sees
// the short-lived authorization code and loopback callback.
func BeginOAuth(ctx context.Context, endpoint string, vault TokenVault) (*OAuthFlow, error) {
	resource, err := url.Parse(endpoint)
	if err != nil || resource.Scheme != "https" || resource.Host == "" {
		return nil, errors.New("Abocado MCP URL must use HTTPS for OAuth")
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resourceMeta, err := fetchResourceMetadata(ctx, client, resource)
	if err != nil {
		return nil, err
	}
	if len(resourceMeta.AuthorizationServers) == 0 {
		return nil, errors.New("Abocado does not advertise an OAuth authorization server")
	}
	authServer, err := url.Parse(resourceMeta.AuthorizationServers[0])
	if err != nil || authServer.Scheme != "https" || authServer.Host == "" {
		return nil, errors.New("Abocado authorization server metadata URL is invalid")
	}
	meta, err := fetchAuthorizationMetadata(ctx, client, authServer)
	if err != nil {
		return nil, err
	}
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" || meta.RegistrationEndpoint == "" {
		return nil, errors.New("Abocado OAuth server must expose authorization, token, and dynamic registration endpoints")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start secure loopback OAuth callback: %w", err)
	}
	callbackURL := "http://" + listener.Addr().String() + "/callback"
	registered, err := registerOAuthClient(ctx, client, meta.RegistrationEndpoint, callbackURL)
	if err != nil {
		listener.Close()
		return nil, err
	}
	stateBytes := make([]byte, 32)
	verifierBytes := make([]byte, 32)
	if _, err = rand.Read(stateBytes); err != nil {
		listener.Close()
		return nil, err
	}
	if _, err = rand.Read(verifierBytes); err != nil {
		listener.Close()
		return nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	authURL, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		listener.Close()
		return nil, err
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", registered.ClientID)
	q.Set("redirect_uri", callbackURL)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("resource", endpoint)
	scopes := resourceMeta.ScopesSupported
	if len(scopes) == 0 {
		scopes = meta.ScopesSupported
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	authURL.RawQuery = q.Encode()
	done := make(chan error, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	var once sync.Once
	finish := func(e error) { once.Do(func() { done <- e; close(done); _ = server.Close() }) }
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state)) != 1 {
			http.Error(w, "OAuth state mismatch", http.StatusBadRequest)
			finish(errors.New("OAuth state validation failed"))
			return
		}
		if providerErr := r.URL.Query().Get("error"); providerErr != "" {
			http.Error(w, "Abocado authorization was not completed", http.StatusBadRequest)
			finish(fmt.Errorf("Abocado OAuth authorization failed: %s", safeOAuthError(providerErr)))
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "authorization code is missing", http.StatusBadRequest)
			finish(errors.New("OAuth callback did not include an authorization code"))
			return
		}
		tokens, e := exchangeOAuthCode(ctx, client, meta.TokenEndpoint, registered.ClientID, callbackURL, code, verifier)
		if e != nil {
			http.Error(w, "Abocado connection failed. Return to the app for details.", http.StatusBadGateway)
			finish(e)
			return
		}
		if e = vault.Save(tokens); e != nil {
			http.Error(w, "Secure credential storage failed.", http.StatusInternalServerError)
			finish(e)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>Connected</title><p>Abocado is connected. You may return to K-Drama Studio.</p>")
		finish(nil)
	})
	go func() {
		e := server.Serve(listener)
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			finish(e)
		}
	}()
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
	go func() {
		select {
		case <-ctxTimeout.Done():
			finish(errors.New("Abocado login timed out"))
		case <-done:
		}
		cancel()
	}()
	flow := &OAuthFlow{URL: authURL.String(), Done: done, stop: func() { finish(errors.New("Abocado login cancelled")); cancel() }}
	return flow, nil
}

func fetchResourceMetadata(ctx context.Context, c *http.Client, resource *url.URL) (protectedResourceMetadata, error) {
	paths := []string{"/.well-known/oauth-protected-resource"}
	clean := strings.Trim(resource.Path, "/")
	if clean != "" {
		paths = append(paths, "/.well-known/oauth-protected-resource/"+clean)
	}
	var last error
	for _, p := range paths {
		u := *resource
		u.Path = p
		u.RawQuery = ""
		u.Fragment = ""
		var m protectedResourceMetadata
		if err := getJSON(ctx, c, u.String(), &m); err == nil {
			return m, nil
		} else {
			last = err
		}
	}
	if last == nil {
		last = errors.New("OAuth resource metadata not available")
	}
	return protectedResourceMetadata{}, fmt.Errorf("discover Abocado OAuth resource metadata: %w", last)
}
func fetchAuthorizationMetadata(ctx context.Context, c *http.Client, server *url.URL) (authorizationMetadata, error) {
	paths := []string{"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration"}
	var last error
	for _, p := range paths {
		u := *server
		u.Path = p
		u.RawQuery = ""
		u.Fragment = ""
		var m authorizationMetadata
		if err := getJSON(ctx, c, u.String(), &m); err == nil {
			return m, nil
		} else {
			last = err
		}
	}
	return authorizationMetadata{}, fmt.Errorf("discover Abocado OAuth server metadata: %w", last)
}
func getJSON(ctx context.Context, c *http.Client, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("OAuth metadata returned HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
func registerOAuthClient(ctx context.Context, c *http.Client, endpoint, redirectURI string) (struct {
	ClientID string `json:"client_id"`
}, error) {
	var out struct {
		ClientID string `json:"client_id"`
	}
	payload, _ := json.Marshal(map[string]any{"client_name": "K-Drama 120 Auto Studio", "redirect_uris": []string{redirectURI}, "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}, "token_endpoint_auth_method": "none"})
	body := strings.NewReader(string(payload))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, fmt.Errorf("OAuth client registration returned HTTP %d", resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	if err != nil {
		return out, err
	}
	if out.ClientID == "" {
		return out, errors.New("OAuth registration returned no client ID")
	}
	return out, nil
}

func (v TokenVault) LoadFresh(ctx context.Context) (OAuthTokens, error) {
	tokens, err := v.Load()
	if err != nil {
		return tokens, err
	}
	if tokens.ExpiresAt.IsZero() || time.Until(tokens.ExpiresAt) > time.Minute {
		return tokens, nil
	}
	if tokens.RefreshToken == "" {
		return tokens, errors.New("Abocado login expired; reconnect from the app")
	}
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {tokens.ClientID}, "refresh_token": {tokens.RefreshToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokens.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokens, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return tokens, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokens, fmt.Errorf("OAuth refresh returned HTTP %d", resp.StatusCode)
	}
	var r tokenReply
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return tokens, err
	}
	if r.AccessToken == "" {
		return tokens, errors.New("OAuth refresh returned no access token")
	}
	tokens.AccessToken = r.AccessToken
	if r.RefreshToken != "" {
		tokens.RefreshToken = r.RefreshToken
	}
	if r.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	return tokens, v.Save(tokens)
}
func exchangeOAuthCode(ctx context.Context, c *http.Client, endpoint, clientID, redirectURI, code, verifier string) (OAuthTokens, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "redirect_uri": {redirectURI}, "code": {code}, "code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthTokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return OAuthTokens{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OAuthTokens{}, fmt.Errorf("OAuth token exchange returned HTTP %d", resp.StatusCode)
	}
	var r tokenReply
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return OAuthTokens{}, err
	}
	if r.AccessToken == "" {
		return OAuthTokens{}, errors.New("OAuth token response is missing access token")
	}
	tokens := OAuthTokens{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, TokenURL: endpoint, ClientID: clientID}
	if r.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	return tokens, nil
}
func safeOAuthError(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 80 {
		return "provider error"
	}
	return v
}

