package configs

import (
	"fmt"

	"github.com/pelletier/go-toml"
)

type Config struct {
	SourceDB SourceConfig `toml:"source_db"`
	TargetDB TargetConfig `toml:"target_db"`
}

type SourceConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DB       string `toml:"db"`
}

type TargetConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DB       string `toml:"db"`
}

func ReadConfigFile(configFile string) (*Config, error) {
	config := &Config{}

	// Open the TOML file
	file, err := toml.LoadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file(%s): %v", configFile, err)
	}

	// Unmarshal the TOML data into the Config struct
	if err := file.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %v", err)
	}

	return config, nil
}
