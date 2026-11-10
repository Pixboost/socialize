package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateDraftForAuthenticatedMember(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization = %q", got)
		}
		switch r.URL.Path {
		case "/v2/userinfo":
			_, _ = w.Write([]byte(`{"sub":"member123"}`))
		case "/v2/ugcPosts":
			var body struct {
				Author         string `json:"author"`
				LifecycleState string `json:"lifecycleState"`
				Content        struct {
					Share struct {
						Commentary struct {
							Text string `json:"text"`
						} `json:"shareCommentary"`
					} `json:"com.linkedin.ugc.ShareContent"`
				} `json:"specificContent"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Author != "urn:li:person:member123" || body.LifecycleState != "DRAFT" || body.Content.Share.Commentary.Text != postText {
				t.Errorf("unexpected draft payload: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"urn:li:share:123"}`))
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := client{httpClient: server.Client(), baseURL: server.URL, token: "test-token"}
	author, err := c.author(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	id, err := c.createDraft(context.Background(), author)
	if err != nil {
		t.Fatal(err)
	}
	if id != "urn:li:share:123" || strings.Join(calls, ",") != "GET /v2/userinfo,POST /v2/ugcPosts" {
		t.Fatalf("id = %q, calls = %v", id, calls)
	}
}

func TestDraftFailureDoesNotPublishOrRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	c := client{httpClient: server.Client(), baseURL: server.URL, token: "test-token"}
	_, err := c.createDraft(context.Background(), "urn:li:person:member123")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || calls != 1 {
		t.Fatalf("error = %v, calls = %d", err, calls)
	}
}

func TestDraftUnknownOutcomeAndHeaderID(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		id      string
		wantErr string
	}{
		{"server error", 500, "", "outcome may be unknown"},
		{"missing id", 201, "", "returned no post ID"},
		{"header id", 201, "urn:li:share:123", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.id != "" {
					w.Header().Set("X-Restli-Id", tc.id)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			c := client{httpClient: server.Client(), baseURL: server.URL, token: "test-token"}
			id, err := c.createDraft(context.Background(), "urn:li:person:member123")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil || id != tc.id {
				t.Fatalf("id = %q, error = %v", id, err)
			}
		})
	}
}

func TestAuthorRequiresSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c := client{httpClient: server.Client(), baseURL: server.URL, token: "test-token"}
	if _, err := c.author(context.Background()); err == nil {
		t.Fatal("expected error for missing subject")
	}
}
