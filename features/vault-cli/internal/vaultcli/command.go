package vaultcli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	arcticvault "github.com/axsh/arctic-tern/shared/libs/go/vault"
)

type SetInput struct {
	Provider string
	Name     string
	Secret   string
}

func SetKey(_ context.Context, in SetInput) error {
	if err := validateProvider(in.Provider); err != nil {
		return err
	}
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(in.Secret) == "" {
		return errors.New("secret is required")
	}
	store := arcticvault.NewKeyringVaultBackend()
	return store.Set(BuildPath(in.Provider, in.Name), in.Secret)
}

func GetKey(_ context.Context, provider string, name string) (string, error) {
	if err := validateProvider(provider); err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "", errors.New("name is required")
	}
	store := arcticvault.NewKeyringVaultBackend()
	ref := fmt.Sprintf("vault://%s", BuildPath(provider, name))
	return store.Resolve(ref)
}

func BuildPath(provider string, name string) string {
	return fmt.Sprintf("providers/%s/%s", strings.TrimSpace(provider), strings.TrimSpace(name))
}

func validateProvider(provider string) error {
	switch strings.TrimSpace(provider) {
	case "openai", "anthropic":
		return nil
	default:
		return errors.New("provider must be openai or anthropic")
	}
}
