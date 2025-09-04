// Copyright 2025 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"os"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

// PluginConfig defines the configuration structure for the golangci-lint plugin.
// The configuration should be defined in the godel/config/check.yml file.
// This configuration is a subset of the golangci-lint configuration, and represents
// the configuration that can be specified by the user. This user-provided configuration
// is merged with a hard-coded base configuration.
type PluginConfig struct {
	Linters LintersConfig `yaml:"linters,omitempty"`
}

type LintersConfig struct {
	Enable     []string         `yaml:"enable,omitempty"`
	Disable    []string         `yaml:"disable,omitempty"`
	Settings   yaml.MapSlice    `yaml:"settings,omitempty"`
	Exclusions ExclusionsConfig `yaml:"exclusions,omitempty"`
}

type ExclusionsConfig struct {
	Rules       []RulesConfig `yaml:"rules,omitempty"`
	Paths       []string      `yaml:"paths,omitempty"`
	PathsExcept []string      `yaml:"paths-except,omitempty"`
}

type RulesConfig struct {
	Linters    []string `yaml:"linters,omitempty"`
	Path       string   `yaml:"path,omitempty"`
	PathExcept string   `yaml:"path-except,omitempty"`
	Text       string   `yaml:"text,omitempty"`
	Source     string   `yaml:"source,omitempty"`
}

func PluginConfigFromFile(configFile string) (*PluginConfig, error) {
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read plugin config file %s", configFile)
	}
	return PluginConfigFromBytes(configBytes)
}

func PluginConfigFromBytes(configBytes []byte) (*PluginConfig, error) {
	var cfg PluginConfig
	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal golangci-lint plugin config")
	}
	return &cfg, nil
}
