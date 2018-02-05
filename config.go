package main

import (
	"gopkg.in/yaml.v2"
	"os"
	"io/ioutil"
)

type Config struct {
	Source      Source      `yaml:"source"`
	Destination Destination `yaml:"destination"`
}

type Source struct {
	Token string `yaml:"token"`
	Repo struct {
		Owner string `yaml:"owner"`
		Name  string `yaml:"name"`
	}
}

type Destination struct {
	Token string `yaml:"token"`
	Repo struct {
		Owner        string            `yaml:"owner"`
		Name         string            `yaml:"name"`
		Private      bool              `yaml:"private"`
		Contributors map[string]string `yaml:"contributors"`
	}
}

func ReadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var config Config
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
