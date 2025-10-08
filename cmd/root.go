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

package cmd

import (
	"github.com/palantir/godel-golangci-lint-plugin/cmd/internal/assetloader"
	"github.com/palantir/godel-golangci-lint-plugin/config"
	"github.com/palantir/godel/v2/framework/pluginapi"
	"github.com/palantir/pkg/cobracli"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	debugFlagVal            bool
	projectDirFlagVal       string
	pluginConfigFileFlagVal string
	godelConfigFileFlagVal  string
	assetsFlagVal           []string
)

var rootCmd = &cobra.Command{
	Use: "godel-golangci-lint-plugin",
}

func Execute() int {
	return cobracli.ExecuteWithDebugVarAndDefaultParams(rootCmd, &debugFlagVal)
}

// NewAssetRunner initializes and returns a GolangCILintAssetRunner. The assets and the exclude and plugin configuration
// are loaded based on flag values, Returns the loaded plugin configuration as well (which may be nil if it is empty).
func NewAssetRunner() (*GolangCILintAssetRunner, *config.PluginConfig, error) {
	assetInfo, err := assetloader.GetAssetInfo(assetsFlagVal)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load assets")
	}

	excludes, err := godelExcludeConfigFromFlags()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load project excludes from godel config")
	}

	pluginConfig, err := pluginConfigFromFlags()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load plugin configuration")
	}

	golangCILintConfig, err := config.DefaultPalantirConfigMergedWithExcludeMatchersAndPluginConfig(assetInfo.ConfigProvidedByAsset, excludes, pluginConfig)
	if err != nil {
		return nil, nil, err
	}

	return NewGolangCILintAssetRunner(assetInfo.GolangCILintAssetPath, golangCILintConfig), pluginConfig, nil
}

func init() {
	pluginapi.AddDebugPFlagPtr(rootCmd.PersistentFlags(), &debugFlagVal)
	pluginapi.AddProjectDirPFlagPtr(rootCmd.PersistentFlags(), &projectDirFlagVal)
	pluginapi.AddConfigPFlagPtr(rootCmd.PersistentFlags(), &pluginConfigFileFlagVal)
	pluginapi.AddGodelConfigPFlagPtr(rootCmd.PersistentFlags(), &godelConfigFileFlagVal)
	pluginapi.AddAssetsPFlagPtr(rootCmd.PersistentFlags(), &assetsFlagVal)
}
