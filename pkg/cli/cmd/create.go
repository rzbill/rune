package cmd

import (
	"fmt"
	"strings"

	"github.com/rzbill/rune/pkg/api/client"
	"github.com/rzbill/rune/pkg/types"
	"github.com/spf13/cobra"
)

// createCmd is the umbrella command for quick create
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create resources quickly",
		Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
	}
	cmd.AddCommand(newCreateSecretCmd())
	cmd.AddCommand(newCreateConfigCmd())
	return cmd
}

func newCreateSecretCmd() *cobra.Command {
	var namespace string
	var dataPairs []string
	cmd := &cobra.Command{
		Use:   "secret <name>",
		Short: "Create a secret from key=value pairs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data := map[string]string{}
			for _, pair := range dataPairs {
				k, v, err := splitPair(pair)
				if err != nil {
					return err
				}
				data[k] = v
			}
			api, err := client.NewClient(client.DefaultClientOptions())
			if err != nil {
				return err
			}
			defer api.Close()
			sc := client.NewSecretClient(api)
			sec := &types.Secret{Name: name, Namespace: namespace, Type: "static", Data: data}
			if err := sc.CreateSecret(sec); err != nil {
				return err
			}
			fmt.Printf("Secret %s/%s created\n", namespace, name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace")
	cmd.Flags().StringSliceVar(&dataPairs, "data", nil, "Data entries key=value (can repeat)")
	return cmd
}

func newCreateConfigCmd() *cobra.Command {
	var namespace string
	var dataPairs []string
	cmd := &cobra.Command{
		Use:   "config <name>",
		Short: "Create a config from key=value pairs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data := map[string]string{}
			for _, pair := range dataPairs {
				k, v, err := splitPair(pair)
				if err != nil {
					return err
				}
				data[k] = v
			}
			api, err := client.NewClient(client.DefaultClientOptions())
			if err != nil {
				return err
			}
			defer api.Close()
			cc := client.NewConfigMapClient(api)
			cfg := &types.ConfigMap{Name: name, Namespace: namespace, Data: data}
			if err := cc.CreateConfigMap(cfg); err != nil {
				return err
			}
			fmt.Printf("Config %s/%s created\n", namespace, name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace")
	cmd.Flags().StringSliceVar(&dataPairs, "data", nil, "Data entries key=value (can repeat)")
	return cmd
}

func splitPair(pair string) (string, string, error) {
	parts := strings.SplitN(pair, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid data format: %s (expected key=value)", pair)
	}
	return parts[0], parts[1], nil
}

func init() { rootCmd.AddCommand(newCreateCmd()) }
