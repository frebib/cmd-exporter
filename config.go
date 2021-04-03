package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Startup *Script
	Scripts []Script
}

func LoadConfig(path string, config *Config) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return yaml.NewDecoder(file).Decode(config)
}
