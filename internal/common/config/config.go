package config

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type CommonConfig struct {
	Verbose    bool
	Quiet      bool
	OutputMode string
	PrintJSON  bool
}

func SetupViper(cmd *cobra.Command, configPath string, envPrefix string) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix(strings.ToUpper(envPrefix))
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	flags := cmd.Flags()
	if err := v.BindPFlags(flags); err != nil {
		return nil, err
	}
	return v, nil
}

func ReadCommon(v *viper.Viper) CommonConfig {
	return CommonConfig{
		Verbose:    v.GetBool("verbose"),
		Quiet:      v.GetBool("quiet"),
		OutputMode: v.GetString("output-mode"),
		PrintJSON:  v.GetBool("print-json"),
	}
}
