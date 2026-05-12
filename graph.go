package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

type GraphClient struct {
	tokenSource oauth2.TokenSource
	httpClient  *http.Client
	userID      string
}

func NewGraphClient(cfg Config, tok *oauth2.Token) (*GraphClient, error) {
	src := oauthConfig(cfg).TokenSource(context.Background(), tok)
	g := &GraphClient{
		tokenSource: src,
		httpClient:  &http.Client{},
	}

	// Fetch user ID for filtering (e.g., pending chats)
	data, err := g.get("/me", url.Values{"$select": {"id"}})
	if err != nil {
		return nil, fmt.Errorf("fetching user profile: %w", err)
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &me); err != nil {
		return nil, err
	}
	g.userID = me.ID

	return g, nil
}

func (g *GraphClient) get(path string, query url.Values) ([]byte, error) {
	u := graphBaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	return g.do(req)
}

func (g *GraphClient) patch(path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PATCH", graphBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = g.do(req)
	return err
}

func (g *GraphClient) post(path string, body interface{}) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", graphBaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return g.do(req)
}

func (g *GraphClient) do(req *http.Request) ([]byte, error) {
	tok, err := g.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("getting token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graph API %s %s returned %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}

	return body, nil
}
