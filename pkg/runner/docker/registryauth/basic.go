package registryauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
)

type BasicTokenConfig struct {
	Registry string
	Username string
	Password string
	Token    string
}

type BasicTokenProvider struct {
	cfg BasicTokenConfig
}

func NewBasicTokenProvider(cfg BasicTokenConfig) *BasicTokenProvider {
	return &BasicTokenProvider{cfg: cfg}
}

func (p *BasicTokenProvider) Match(host string) bool {
	return hostMatches(p.cfg.Registry, host)
}

func (p *BasicTokenProvider) Resolve(ctx context.Context, host, imageRef string) (string, error) {
	username := p.cfg.Username
	password := p.cfg.Password
	if username == "" && password == "" && p.cfg.Token != "" {
		username = "token"
		password = p.cfg.Token
	}
	if username == "" || password == "" {
		return "", nil
	}
	payload := map[string]string{
		"username":      username,
		"password":      password,
		"serveraddress": host,
	}
	b, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(b), nil
}
