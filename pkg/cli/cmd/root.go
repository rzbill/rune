package cmd

import (
	"fmt"
	"os"

	"github.com/rzbill/rune/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rune",
	Short: "Rune - Lightweight Orchestration Platform",
	Long: `Rune is a lightweight, single-binary orchestration platform 
inspired by Kubernetes and Nomad. It is designed for developers 
who want to deploy and scale services quickly and intuitively, 
without unnecessary complexity.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is specified, display the help
		if len(args) == 0 {
			cmd.Help()
			return
		}
	},
	Version: version.Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.rune/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add global environment variables
	viper.SetEnvPrefix("RUNE")
	viper.AutomaticEnv() // read in environment variables that match

	// Register deps command group
	rootCmd.AddCommand(newDepsCmd())
	// Register auth-related commands
	rootCmd.AddCommand(newLoginCmd())
	rootCmd.AddCommand(newWhoAmICmd())
	rootCmd.AddCommand(newTokenCmd())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".rune" (without extension).
		viper.AddConfigPath(home + "/.rune")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

// getEnv is a small wrapper so other commands can mock env lookup in tests
func getEnv(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	return v, ok
}
