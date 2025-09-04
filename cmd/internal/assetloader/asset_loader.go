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

package assetloader

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type AssetInfo struct {
	// path to the golangci-lint asset. Must be non-empty.
	GolangCILintAssetPath string

	// the configuration provided by the configuration asset. May be nil if no configuration asset was provided.
	ConfigProvidedByAsset []byte
}

// GetAssetInfo verifies that the provided assets are valid and returns an AssetInfo that is properly populated.
//
// The provided assets must contain exactly 1 golangci-lint asset.
// The provided assets may contain at most 1 config asset.
func GetAssetInfo(assets []string) (AssetInfo, error) {
	var (
		// stores valid assets
		golangCILintAssets, configAssets []string

		// value returned by the latest valid config asset considered
		configFromAsset []byte

		// errors encountered while trying to load assets
		golangCILintAssetErrors, configAssetErrors []error
	)

	for _, currAsset := range assets {
		if err := verifyGolangCILintAsset(currAsset); err == nil {
			golangCILintAssets = append(golangCILintAssets, currAsset)
		} else {
			golangCILintAssetErrors = append(golangCILintAssetErrors, err)
		}

		if config, err := verifyConfigAsset(currAsset); err == nil {
			configAssets = append(configAssets, currAsset)
			configFromAsset = config
		} else {
			configAssetErrors = append(configAssetErrors, err)
		}
	}

	var assetInfo AssetInfo

	// verify that there is exactly 1 golangci-lint asset
	switch numGolangCILintAssets := len(golangCILintAssets); numGolangCILintAssets {
	case 1:
		// exactly 1 golangci-lint asset found
		assetInfo.GolangCILintAssetPath = golangCILintAssets[0]
	case 0:
		// no golangci-lint assets found
		return assetInfo, wrapOrNewError(fmt.Sprintf("plugin must be configured with a single golangci-lint asset, but none was found in assets %v", assets), golangCILintAssetErrors)
	default:
		// multiple golangci-lint assets found
		return assetInfo, pkgerrors.New(fmt.Sprintf("plugin must must be configured with exactly 1 golangci-lint asset, but got %d: %v", numGolangCILintAssets, golangCILintAssets))
	}

	// verify that at most 1 config asset is specified
	if numConfigAssets := len(configAssets); numConfigAssets > 0 {
		if numConfigAssets > 1 {
			return assetInfo, pkgerrors.New(fmt.Sprintf("plugin must must be configured with at most 1 config asset, but got %d: %v", numConfigAssets, configAssets))
		}
		assetInfo.ConfigProvidedByAsset = configFromAsset
	}

	return assetInfo, nil
}

func getAssetOutput(assetPath string, args ...string) ([]byte, error) {
	cmd := exec.Command(assetPath, args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to run command \"%s\", output: %s", strings.Join(cmd.Args, " "), string(outputBytes))
	}
	return outputBytes, nil
}

// verifyGolangCILintAsset returns a nil error if the executable at the provided path is a golangci-lint executable,
// false otherwise. Makes the determination by running the executable with the "--version" argument and verifying that
// the output matches expectations.
func verifyGolangCILintAsset(assetPath string) error {
	args := []string{"--version"}
	outputBytes, err := getAssetOutput(assetPath, args...)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(outputBytes, []byte("golangci-lint has version ")) {
		return pkgerrors.New(fmt.Sprintf("expected output of command \"%s\" to start with 'golangci-lint has version ', but got: %q", strings.Join(args, " "), outputBytes))
	}
	return nil
}

func verifyConfigAsset(assetPath string) ([]byte, error) {
	assetContent, err := os.ReadFile(assetPath)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to read config asset at %s", assetPath)
	}
	var obj any
	if err := yaml.Unmarshal(assetContent, &obj); err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to unmarshal golangci-lint-config asset as YAML")
	}
	return assetContent, nil
}

func wrapOrNewError(msg string, errs []error) error {
	if err := errors.Join(errs...); err != nil {
		return pkgerrors.Wrapf(err, "%s", msg)
	}
	return pkgerrors.New(msg)
}
