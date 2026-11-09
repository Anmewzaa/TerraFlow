package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type OIDCProvider struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
}

func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (*OIDCProvider, error) {
	issuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", cfg.TenantID)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc.NewProvider: %w", err)
	}

	return &OIDCProvider{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURI,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// AuthCodeURL builds the authorize URL with state, nonce and a PKCE (S256)
// challenge derived from verifier.
func (p *OIDCProvider) AuthCodeURL(state, nonce, verifier string) string {
	return p.oauth.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("response_mode", "query"),
		oauth2.S256ChallengeOption(verifier),
	)
}

// Exchange trades the code for tokens, proving possession of the PKCE verifier.
func (p *OIDCProvider) Exchange(ctx context.Context, code, verifier string) (*oidc.IDToken, error) {
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("oauth2 exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}
	return p.verifier.Verify(ctx, rawIDToken)
}
