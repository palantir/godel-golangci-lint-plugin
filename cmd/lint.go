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
	"slices"

	"github.com/palantir/godel-golangci-lint-plugin/config"
	"github.com/palantir/godel-golangci-lint-plugin/internal/compiles"
	godelconfig "github.com/palantir/godel/v2/framework/godel/config"
	"github.com/palantir/pkg/matcher"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const compilesCheckName = "compiles"

var (
	disableCompilesCheckFlagVal bool
	fixFlagVal                  bool

	lintCmd = &cobra.Command{
		Use:   "lint [flags] [checks]",
		Short: "Run linters (runs all linters if none are specified)",
		RunE: func(cmd *cobra.Command, args []string) error {
			assetRunner, pluginConfig, err := NewAssetRunner()
			if err != nil {
				return errors.Wrap(err, "failed to initialize golangci-lint asset runner")
			}

			// if plugin is disabled, do nothing
			if pluginConfig != nil && pluginConfig.Disable {
				return nil
			}

			// if no checks were specified or "compiles" was specified, run compiles check first.
			// If check fails, exit with the exit code.
			// If check succeeds and "compiles" was specified as an argument, remove it from the arguments (it is not
			// an actual check).
			if compilesCheckArgIdx := slices.Index(args, compilesCheckName); len(args) == 0 || compilesCheckArgIdx != -1 {
				// only run compiles check if it is not disabled by flag.
				// not a conditional on the full block because the logic to remove "compiles" from args should still run.
				if !disableCompilesCheckFlagVal {
					// this should match the value that is used by golangci-lint. The current construction of the plugin
					// invokes golangci-lint without any arguments, which causes it to use "./...".
					const pkgSelector = "./..."
					if compilesRunResult := compiles.Run([]string{pkgSelector}, debugFlagVal, cmd.OutOrStdout()); compilesRunResult != 0 {
						os.Exit(compilesRunResult)
					}
				}

				if compilesCheckArgIdx != -1 {
					// remove "compiles" argument if it was provided
					args = slices.Delete(args, compilesCheckArgIdx, compilesCheckArgIdx+1)

					// if "compiles" was the only argument, exit successfully
					if len(args) == 0 {
						// Print success output. Only do this if exiting at this point because otherwise golangci-lint
						// will print its own output.
						compiles.SuccessOutput(cmd.OutOrStdout())
						return nil
					}
				}
			}

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

			return runDelegatedGolangCILintCommand(assetRunner, preConfigArgs, postConfigArgs, cmd.OutOrStdout(), cmd.ErrOrStderr(), debugFlagVal)
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
func runDelegatedGolangCILintCommand(assetRunner *GolangCILintAssetRunner, preConfigArgs, postConfigArgs []string, stdout, stderr io.Writer, debugMode bool) error {
	exitCode, err := assetRunner.RunGolangCILintWithConfig(preConfigArgs, postConfigArgs, stderr, stdout, debugMode)
	if err != nil {
		return err
	}

	// use os.Exit because full control is delegated to the golangci-lint process and exit code of this process should
	// match the exit code of that process. Note that no deferred functions will run after this call.
	os.Exit(exitCode)

	return nil
}

func godelExcludeConfigFromFlags() (matcher.NamesPathsCfg, error) {
	return godelconfig.ReadGodelConfigExcludesFromFile(godelConfigFileFlagVal)
}

func pluginConfigFromFlags() (*config.PluginConfig, error) {
	pluginConfig, err := config.PluginConfigFromFile(pluginConfigFileFlagVal)
	if err != nil {
		// if plugin config does not exist, continue with nil config
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return pluginConfig, nil
}

func init() {
	lintCmd.Flags().BoolVarP(&disableCompilesCheckFlagVal, "disable-compiles-check", "", false, "Disable the compiles check that the plugin runs before invoking linters")
	lintCmd.Flags().BoolVarP(&fixFlagVal, "fix", "", false, "Fix found issues (if it's supported by the linter)")

	rootCmd.AddCommand(lintCmd)
}
