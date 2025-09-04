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
	"github.com/palantir/godel/v2/framework/pluginapi"
	"github.com/palantir/pkg/cobracli"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	debugFlagVal            bool
	projectDirFlagVal       string
	pluginConfigFileFlagVal string
	godelConfigFileFlagVal  string
	assetsFlagVal           []string

	// Package-level variable that is set by InitAssetCmds.
	// Is guaranteed to be set and valid after InitAssetCmds is called (if it is not valid, an error is returned and
	// the program will not run).
	assetRunner *GolangCILintAssetRunner
)

var rootCmd = &cobra.Command{
	Use: "godel-golangci-lint-plugin",
}

func Execute() int {
	return cobracli.ExecuteWithDebugVarAndDefaultParams(rootCmd, &debugFlagVal)
}

func InitAssetCmds(args []string) error {
	if _, _, err := rootCmd.Traverse(args); err != nil && err != pflag.ErrHelp {
		return errors.Wrapf(err, "failed to parse arguments")
	}

	assetInfo, err := assetloader.GetAssetInfo(assetsFlagVal)
	if err != nil {
		return err
	}

	golangCILintConfig, err := getConfig(assetInfo.ConfigProvidedByAsset)
	if err != nil {
		return err
	}

	assetRunner = NewGolangCILintAssetRunner(assetInfo.GolangCILintAssetPath, golangCILintConfig)
	return nil
}

func init() {
	pluginapi.AddDebugPFlagPtr(rootCmd.PersistentFlags(), &debugFlagVal)
	pluginapi.AddProjectDirPFlagPtr(rootCmd.PersistentFlags(), &projectDirFlagVal)
	pluginapi.AddConfigPFlagPtr(rootCmd.PersistentFlags(), &pluginConfigFileFlagVal)
	pluginapi.AddGodelConfigPFlagPtr(rootCmd.PersistentFlags(), &godelConfigFileFlagVal)
	pluginapi.AddAssetsPFlagPtr(rootCmd.PersistentFlags(), &assetsFlagVal)
}
