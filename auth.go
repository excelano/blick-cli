package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

func getScopes(cfg Config) []string {
	s := []string{
		"User.Read",
		"Mail.ReadWrite",
		"Mail.Send",
		"Calendars.Read",
		"offline_access",
	}
	if cfg.EnableTeams {
		s = append(s, "Chat.ReadWrite")
	}
	return s
}

func oauthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID: cfg.ClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", cfg.TenantID),
			TokenURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", cfg.TenantID),
		},
		Scopes: getScopes(cfg),
	}
}

func tokenPath() string {
	return filepath.Join(configDir(), "token.json")
}

func authenticate(cfg Config) (*oauth2.Token, error) {
	tok, err := loadCachedToken()
	if err == nil {
		src := oauthConfig(cfg).TokenSource(context.Background(), tok)
		refreshed, err := src.Token()
		if err == nil {
			if refreshed.AccessToken != tok.AccessToken {
				_ = saveCachedToken(refreshed)
			}
			return refreshed, nil
		}
		// Refresh failed, fall through to device code flow
	}

	tok, err = deviceCodeFlow(cfg)
	if err != nil {
		return nil, err
	}

	if err := saveCachedToken(tok); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not cache token: %v\n", err)
	}

	return tok, nil
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func deviceCodeFlow(cfg Config) (*oauth2.Token, error) {
	deviceURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", cfg.TenantID)
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", cfg.TenantID)

	activeScopes := getScopes(cfg)
	scopeStr := ""
	for i, s := range activeScopes {
		if i > 0 {
			scopeStr += " "
		}
		scopeStr += s
	}

	resp, err := http.PostForm(deviceURL, url.Values{
		"client_id": {cfg.ClientID},
		"scope":     {scopeStr},
	})
	if err != nil {
		return nil, fmt.Errorf("device code request: %w", err)
	}
	defer resp.Body.Close()

	var dcResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("parsing device code response: %w", err)
	}

	if dcResp.DeviceCode == "" {
		return nil, fmt.Errorf("no device code returned (check client_id and tenant_id)")
	}

	fmt.Printf("\nTo sign in, open a browser and go to:\n  %s\n\nEnter code: %s\n\nWaiting for authentication...\n", dcResp.VerificationURI, dcResp.UserCode)

	interval := time.Duration(dcResp.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dcResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		tok, err := pollToken(tokenURL, cfg.ClientID, dcResp.DeviceCode)
		if err == errPending {
			continue
		}
		if err == errSlowDown {
			interval += 5 * time.Second
			continue
		}
		if err != nil {
			return nil, err
		}

		fmt.Println("Authenticated successfully!")
		return tok, nil
	}

	return nil, fmt.Errorf("device code expired, please try again")
}

var (
	errPending  = fmt.Errorf("authorization pending")
	errSlowDown = fmt.Errorf("slow down")
)

func pollToken(tokenURL, clientID, deviceCode string) (*oauth2.Token, error) {
	resp, err := http.PostForm(tokenURL, url.Values{
		"client_id":   {clientID},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
	})
	if err != nil {
		return nil, fmt.Errorf("token poll: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	switch result.Error {
	case "authorization_pending":
		return nil, errPending
	case "slow_down":
		return nil, errSlowDown
	case "":
		// Success
	default:
		return nil, fmt.Errorf("auth error: %s: %s", result.Error, result.ErrorDesc)
	}

	return &oauth2.Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		Expiry:       time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

func loadCachedToken() (*oauth2.Token, error) {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveCachedToken(tok *oauth2.Token) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenPath(), data, 0600)
}
