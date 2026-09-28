// Author: David M. Anderson
// Built with AI assistance (Claude, Anthropic)

package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/excelano/atrest"
	"golang.org/x/oauth2"
)

// TestTokenCacheSealsForReal drives saveCachedToken and loadCachedToken
// through the real atrest package, not a stand-in, so a wiring mistake — the
// wrong seal name, an argument swapped — shows up here rather than only
// after a release. What it can prove depends on what this machine actually
// offers: a machine with a reachable key store proves sealing works, and one
// offering none proves the fallback is exactly the pre-sealing behaviour.
func TestTokenCacheSealsForReal(t *testing.T) {
	withTempHome(t)
	want := &oauth2.Token{
		AccessToken:  "at",
		RefreshToken: "rt",
		Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
	}
	if err := saveCachedToken(want); err != nil {
		t.Fatalf("saveCachedToken: %v", err)
	}

	stored, err := os.ReadFile(tokenPath())
	if err != nil {
		t.Fatal(err)
	}
	plain, sealed, err := atrest.Open(tokenSealName, stored)
	if err != nil {
		t.Fatalf("atrest.Open of what saveCachedToken wrote: %v", err)
	}
	t.Logf("on this machine, saveCachedToken sealed=%v (stored: %s)", sealed, stored)
	var gotPlain oauth2.Token
	if err := json.Unmarshal(plain, &gotPlain); err != nil {
		t.Fatalf("plaintext under the envelope does not parse: %v", err)
	}
	if gotPlain.RefreshToken != want.RefreshToken {
		t.Errorf("plaintext refresh token = %q, want %q", gotPlain.RefreshToken, want.RefreshToken)
	}

	got, err := loadCachedToken()
	if err != nil {
		t.Fatalf("loadCachedToken: %v", err)
	}
	if got.RefreshToken != want.RefreshToken || !got.Expiry.Equal(want.Expiry) {
		t.Errorf("loadCachedToken = %+v, want %+v", got, want)
	}
}

// A cache this machine cannot open must read back exactly like a missing
// one, since authenticate's only check is err == nil before falling through
// to device code.
func TestLoadCachedTokenUnopenableIsLikeMissing(t *testing.T) {
	withTempHome(t)
	if err := os.MkdirAll(configDir(), 0700); err != nil {
		t.Fatal(err)
	}
	foreign := `{"atrest":1,"alg":"aes-gcm","data":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`
	if err := os.WriteFile(tokenPath(), []byte(foreign), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCachedToken(); err == nil {
		t.Error("loadCachedToken of an unopenable cache returned no error; authenticate would try to use a nil token")
	}
}

func TestLoadCachedTokenMissingFile(t *testing.T) {
	withTempHome(t)
	if _, err := loadCachedToken(); err == nil {
		t.Error("loadCachedToken with no file returned no error")
	}
}
