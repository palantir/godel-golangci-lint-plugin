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
	"io"
	"os"

	"github.com/palantir/godel-golangci-lint-plugin/config"
	godelconfig "github.com/palantir/godel/v2/framework/godel/config"
	"github.com/palantir/pkg/matcher"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	fixFlagVal bool

	lintCmd = &cobra.Command{
		Use:   "lint [flags] [checks]",
		Short: "Run linters (runs all linters if none are specified)",
		RunE: func(cmd *cobra.Command, args []string) error {
			preConfigArgs := []string{
				"run",
			}

			// enable verbose logging if debug flag is set
			if debugFlagVal {
				preConfigArgs = append(preConfigArgs, "-v")
			}

			var postConfigArgs []string
			if len(args) > 0 {
				postConfigArgs = append([]string{
					"--enable-only",
				}, args...)
			}
			if fixFlagVal {
				postConfigArgs = append(postConfigArgs, "--fix")
			}

			return runDelegatedGolangCILintCommand(preConfigArgs, postConfigArgs, cmd.OutOrStdout(), cmd.ErrOrStderr(), debugFlagVal)
		},
	}
)

// runDelegatedGolangCILintCommand runs the golangci-lint executable (asset) with the plugin configuration specified as
// a flag (written to a temporary file and then referenced via flag) and the provided arguments provided before and
// after the configuration flag. The provided stdout and stderr are used. This function returns an error only if it
// fails to write/set up the configuration file: otherwise, it delegates to the asset runner and exits the process using
// the same exit code as the asset runner process. As such, this function should be considered terminal: there should be
// no expectation that this function returns control to the caller (including for running deferred functions or
// cleanup).
func runDelegatedGolangCILintCommand(preConfigArgs, postConfigArgs []string, stdout, stderr io.Writer, debugMode bool) error {
	exitCode, err := assetRunner.RunGolangCILintWithConfig(preConfigArgs, postConfigArgs, stderr, stdout, debugMode)
	if err != nil {
		return err
	}

	// use os.Exit because full control is delegated to the golangci-lint process and exit code of this process should
	// match the exit code of that process. Note that no deferred functions will run after this call.
	os.Exit(exitCode)

	return nil
}

func getConfig(baseConfig []byte) (config.GolangCILintConfig, error) {
	excludes, pluginConfig, err := projectParamFromFlags()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read project excludes from flags")
	}
	return config.DefaultPalantirConfigMergedWithExcludeMatchersAndPluginConfig(baseConfig, excludes, pluginConfig)
}

func projectParamFromFlags() (matcher.NamesPathsCfg, *config.PluginConfig, error) {
	godelExcludeConfig, err := godelconfig.ReadGodelConfigExcludesFromFile(godelConfigFileFlagVal)
	if err != nil {
		return godelExcludeConfig, nil, err
	}

	pluginConfig, err := config.PluginConfigFromFile(pluginConfigFileFlagVal)
	if err != nil {
		return godelExcludeConfig, nil, err
	}
	return godelExcludeConfig, pluginConfig, nil
}

func init() {
	lintCmd.Flags().BoolVarP(&fixFlagVal, "fix", "", false, "Fix found issues (if it's supported by the linter)")

	rootCmd.AddCommand(lintCmd)
}
