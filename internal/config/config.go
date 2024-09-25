package config

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Load config from file into the config struct, config must be a pointer to the config struct
func Load(file string, config any) error {
	v := viper.New()
	m := make(map[string]any)

	if err := mapstructure.Decode(config, &m); err != nil {
		return fmt.Errorf("mapstructure: %v", err)
	}

	if err := v.MergeConfigMap(m); err != nil {
		return fmt.Errorf("merge config map: %v", err)
	}

	v.SetConfigFile(file)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config from file %s: %v", file, err)
	}
	if err := v.Unmarshal(config); err != nil {
		return fmt.Errorf("unmarshal config: %v", err)
	}

	return nil
}
