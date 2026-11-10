package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiBase  = "https://api.linkedin.com"
	postText = "Hello LinkedIn! This is a draft created by Socialize."
)

type client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	token := strings.TrimSpace(os.Getenv("LINKEDIN_ACCESS_TOKEN"))
	if token == "" {
		return errors.New("set LINKEDIN_ACCESS_TOKEN to a member OAuth access token")
	}

	c := client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL: apiBase,
		token:   token,
	}

	ctx := context.Background()
	author, err := c.author(ctx)
	if err != nil {
		return err
	}
	id, err := c.createDraft(ctx, author)
	if err != nil {
		return err
	}
	fmt.Printf("LinkedIn accepted the draft request.\nAuthor: %s\nPost ID: %s\n", author, id)
	fmt.Println("Check LinkedIn to see whether the draft appears in your account.")
	return nil
}

func (c client) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create LinkedIn request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c client) author(ctx context.Context) (string, error) {
	resp, err := c.request(ctx, http.MethodGet, "/v2/userinfo", nil)
	if err != nil {
		return "", fmt.Errorf("retrieve LinkedIn profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("retrieve LinkedIn profile: HTTP %d; check your token and OpenID Connect permissions", resp.StatusCode)
	}
	var profile struct {
		Subject string `json:"sub"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&profile); err != nil {
		return "", fmt.Errorf("decode LinkedIn profile: %w", err)
	}
	if profile.Subject == "" || strings.ContainsAny(profile.Subject, " \t\r\n:") {
		return "", errors.New("LinkedIn profile did not return a valid member ID")
	}
	return "urn:li:person:" + profile.Subject, nil
}

func (c client) createDraft(ctx context.Context, author string) (string, error) {
	payload := map[string]any{
		"author":         author,
		"lifecycleState": "DRAFT",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": map[string]any{
				"shareCommentary":    map[string]string{"text": postText},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]string{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode draft: %w", err)
	}
	resp, err := c.request(ctx, http.MethodPost, "/v2/ugcPosts", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("draft request failed; outcome may be unknown, check LinkedIn before retrying: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode < 400 || resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= 500 {
			return "", fmt.Errorf("LinkedIn returned HTTP %d; outcome may be unknown, check LinkedIn before retrying", resp.StatusCode)
		}
		return "", fmt.Errorf("LinkedIn rejected the draft request (HTTP %d); check API access, permissions, and DRAFT support", resp.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("LinkedIn accepted the request but its response could not be decoded; check LinkedIn before retrying: %w", err)
	}
	if result.ID == "" {
		result.ID = resp.Header.Get("X-Restli-Id")
	}
	if result.ID == "" {
		return "", errors.New("LinkedIn accepted the request but returned no post ID; check LinkedIn before retrying")
	}
	return result.ID, nil
}
