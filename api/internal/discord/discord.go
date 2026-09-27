// Package discord implements the Discord OAuth2 authorization-code flow (scope identify).
package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type User struct {
	ID       string
	Username string
}

// Provider is the part of Discord the API needs; tests substitute a fake.
type Provider interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (User, error)
}

type OAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	BaseURL      string // defaults to https://discord.com
	HTTP         *http.Client
}

func (o *OAuth) base() string {
	if o.BaseURL != "" {
		return strings.TrimRight(o.BaseURL, "/")
	}
	return "https://discord.com"
}

func (o *OAuth) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (o *OAuth) AuthURL(state string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {o.ClientID},
		"scope":         {"identify"},
		"state":         {state},
		"redirect_uri":  {o.RedirectURI},
		"prompt":        {"none"},
	}
	return o.base() + "/oauth2/authorize?" + q.Encode()
}

// Exchange trades an authorization code for the user's Discord identity.
func (o *OAuth) Exchange(ctx context.Context, code string) (User, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {o.RedirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/api/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return User{}, err
	}
	req.SetBasicAuth(o.ClientID, o.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := o.do(req, &tok); err != nil {
		return User{}, fmt.Errorf("discord token: %w", err)
	}
	if tok.AccessToken == "" {
		return User{}, fmt.Errorf("discord token: empty access token")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, o.base()+"/api/users/@me", nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	var me struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := o.do(req, &me); err != nil {
		return User{}, fmt.Errorf("discord user: %w", err)
	}
	if me.ID == "" {
		return User{}, fmt.Errorf("discord user: missing id")
	}
	return User{ID: me.ID, Username: me.Username}, nil
}

func (o *OAuth) do(req *http.Request, out any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := o.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s", resp.Status)
	}
	return json.Unmarshal(body, out)
}
