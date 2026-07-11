# vault-cli

CLI utility for storing and reading provider API keys in OS keyring using `arctic-tern` vault backend.

## Commands

```bash
go run ./cmd/vault-cli set --provider openai --name default --secret "YOUR_KEY"
go run ./cmd/vault-cli set --provider anthropic --name default --secret "YOUR_KEY"
go run ./cmd/vault-cli get --provider openai --name default
```

## Vault Key Mapping

- OpenAI default key: `vault://providers/openai/default`
- Anthropic default key: `vault://providers/anthropic/default`
