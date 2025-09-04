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

	"github.com/palantir/godel-golangci-lint-plugin/config"
	"github.com/palantir/godel-golangci-lint-plugin/runner"
)

type GolangCILintAssetRunner struct {
	golangCILintAssetPath string
	assetConfig           config.GolangCILintConfig
}

func NewGolangCILintAssetRunner(golangCILintAssetPath string, assetConfig config.GolangCILintConfig) *GolangCILintAssetRunner {
	return &GolangCILintAssetRunner{
		golangCILintAssetPath: golangCILintAssetPath,
		assetConfig:           assetConfig,
	}
}

func (r *GolangCILintAssetRunner) Config() config.GolangCILintConfig {
	return r.assetConfig
}

func (r *GolangCILintAssetRunner) RunGolangCILint(args []string, stdout, stderr io.Writer, debugMode bool) int {
	return runner.RunGolangCILint(r.golangCILintAssetPath, args, stdout, stderr, debugMode)
}

func (r *GolangCILintAssetRunner) RunGolangCILintWithConfig(preConfigArgs, postConfigArgs []string, stdout, stderr io.Writer, debugMode bool) (int, error) {
	return runner.RunGolangCILintWithConfig(r.golangCILintAssetPath, preConfigArgs, postConfigArgs, r.assetConfig, stdout, stderr, debugMode)
}
