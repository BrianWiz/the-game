// Package github is a small GitHub REST client that authenticates as a GitHub App
// installation, plus webhook signature checks. It covers only what the bot uses.
package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// App authenticates as one installation of a GitHub App on one repository.
type App struct {
	ClientID string // the App's client ID (or numeric App ID); the JWT issuer
	Key      *rsa.PrivateKey
	Repo     string // "owner/name"
	BaseURL  string // defaults to https://api.github.com
	HTTP     *http.Client
	Now      func() time.Time

	mu           sync.Mutex
	installation int64
	token        string
	tokenExpiry  time.Time
}

// ParseKey reads a PEM RSA private key (PKCS#1, as GitHub issues them, or PKCS#8).
func ParseKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA key")
	}
	return rk, nil
}

func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *App) base() string {
	if a.BaseURL != "" {
		return strings.TrimRight(a.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (a *App) client() *http.Client {
	if a.HTTP != nil {
		return a.HTTP
	}
	return http.DefaultClient
}

// JWT returns a short-lived App JWT (RS256), backdated for clock skew.
func (a *App) JWT() (string, error) {
	now := a.now()
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.ClientID,
	})
	if err != nil {
		return "", err
	}
	signing := header + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(nil, a.Key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}

// Token returns an installation token, refreshed five minutes before it expires.
func (a *App) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && a.now().Before(a.tokenExpiry.Add(-5*time.Minute)) {
		return a.token, nil
	}
	jwt, err := a.JWT()
	if err != nil {
		return "", err
	}
	if a.installation == 0 {
		var inst struct {
			ID int64 `json:"id"`
		}
		if err := a.do(ctx, "Bearer "+jwt, http.MethodGet, "/repos/"+a.Repo+"/installation", nil, &inst); err != nil {
			return "", fmt.Errorf("find installation: %w", err)
		}
		a.installation = inst.ID
	}
	var tok struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	path := fmt.Sprintf("/app/installations/%d/access_tokens", a.installation)
	if err := a.do(ctx, "Bearer "+jwt, http.MethodPost, path, map[string]any{}, &tok); err != nil {
		return "", fmt.Errorf("installation token: %w", err)
	}
	a.token, a.tokenExpiry = tok.Token, tok.ExpiresAt
	return a.token, nil
}

// APIError is a non-2xx answer from GitHub.
type APIError struct {
	Status  int
	Method  string
	Path    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
}

func (a *App) do(ctx context.Context, auth, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base()+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", auth)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		var e struct {
			Message string `json:"message"`
		}
		json.Unmarshal(data, &e)
		return &APIError{Status: resp.StatusCode, Method: method, Path: path, Message: e.Message}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// call makes an authenticated installation request against the repository.
func (a *App) call(ctx context.Context, method, path string, body, out any) error {
	tok, err := a.Token(ctx)
	if err != nil {
		return err
	}
	return a.do(ctx, "token "+tok, method, "/repos/"+a.Repo+path, body, out)
}

// Issue is the subset of an issue the bot reads.
type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
}

// CreateIssue opens an issue in the repository.
func (a *App) CreateIssue(ctx context.Context, title, body string) (Issue, error) {
	var is Issue
	err := a.call(ctx, http.MethodPost, "/issues", map[string]any{"title": title, "body": body}, &is)
	return is, err
}

// Dispatch starts workflow (a file name such as "agent.yml") on ref with inputs.
func (a *App) Dispatch(ctx context.Context, workflow, ref string, inputs map[string]string) error {
	return a.call(ctx, http.MethodPost, "/actions/workflows/"+url.PathEscape(workflow)+"/dispatches",
		map[string]any{"ref": ref, "inputs": inputs}, nil)
}

// WorkflowRun is the subset of a workflow run the bot reads. It is also the
// workflow_run webhook's "workflow_run" object.
type WorkflowRun struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DisplayTitle string `json:"display_title"`
	Path         string `json:"path"` // ".github/workflows/agent.yml"
	Event        string `json:"event"`
	Status       string `json:"status"`     // queued, in_progress, completed, ...
	Conclusion   string `json:"conclusion"` // success, failure, cancelled, ...
	HeadBranch   string `json:"head_branch"`
	HeadSHA      string `json:"head_sha"`
	HTMLURL      string `json:"html_url"`
}

// WorkflowRuns lists runs of workflow created at or after since, newest first.
func (a *App) WorkflowRuns(ctx context.Context, workflow string, since time.Time) ([]WorkflowRun, error) {
	q := url.Values{"per_page": {"100"}, "created": {">=" + since.UTC().Format(time.RFC3339)}}
	var out struct {
		Runs []WorkflowRun `json:"workflow_runs"`
	}
	err := a.call(ctx, http.MethodGet, "/actions/workflows/"+url.PathEscape(workflow)+"/runs?"+q.Encode(), nil, &out)
	return out.Runs, err
}

// User is a comment author.
type User struct {
	Login string `json:"login"`
	Type  string `json:"type"` // "User" or "Bot"
}

// Comment is an issue or PR conversation comment.
type Comment struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	User    User   `json:"user"`
}

// Comments lists the comments on issue or PR n updated at or after since, oldest first.
func (a *App) Comments(ctx context.Context, n int, since time.Time) ([]Comment, error) {
	q := url.Values{"per_page": {"100"}, "since": {since.UTC().Format(time.RFC3339)}}
	var out []Comment
	err := a.call(ctx, http.MethodGet, fmt.Sprintf("/issues/%d/comments?%s", n, q.Encode()), nil, &out)
	return out, err
}
