package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/axsh/entext/features/vault-cli/internal/vaultcli"
	"github.com/spf13/cobra"
)

func main() {
	cmd := newRootCmd(os.Stdin, os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault-cli",
		Short: "Manage keyring secrets for arctic-tern",
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newSetCmd(stdin))
	cmd.AddCommand(newGetCmd(stdout))
	return cmd
}

func newSetCmd(stdin io.Reader) *cobra.Command {
	var provider string
	var name string
	var secret string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set a key in keyring",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(secret) == "" {
				reader := bufio.NewReader(stdin)
				raw, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return err
				}
				secret = strings.TrimSpace(raw)
			}
			return vaultcli.SetKey(context.Background(), vaultcli.SetInput{
				Provider: provider,
				Name:     name,
				Secret:   secret,
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&provider, "provider", "", "Provider name: openai|anthropic")
	flags.StringVar(&name, "name", "default", "Key name")
	flags.StringVar(&secret, "secret", "", "Secret value (or provide from stdin)")
	_ = cmd.MarkFlagRequired("provider")
	return cmd
}

func newGetCmd(stdout io.Writer) *cobra.Command {
	var provider string
	var name string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a key from keyring",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := vaultcli.GetKey(context.Background(), provider, name)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(stdout, v)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&provider, "provider", "", "Provider name: openai|anthropic")
	flags.StringVar(&name, "name", "default", "Key name")
	_ = cmd.MarkFlagRequired("provider")
	return cmd
}
