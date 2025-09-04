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
	"bytes"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/palantir/pkg/matcher"
	"github.com/palantir/pkg/yamlpatch/goccyyamlpatcher"
	"github.com/palantir/pkg/yamlpatch/yamlpatch"
	"github.com/pkg/errors"
)

// GolangCILintConfig represents configuration for golangci-lint.
// This type is the YAML byte slice of configuration that can be read by golangci-lint.
type GolangCILintConfig []byte

// MergeExcludeMatchersWithConfig returns a version of the provided GolangCILintConfig that adds the exclusions
// configuration that corresponds to the provided matchers to the "linters.exclusions.paths" section of the config. That
// section is created in the config if it does not exist; otherwise, the exclude entries are added to the existing
// entries.
func MergeExcludeMatchersWithConfig(configBytes GolangCILintConfig, matchers matcher.NamesPathsCfg) (GolangCILintConfig, error) {
	return MergePluginConfigWithConfig(configBytes, convertNamesPathConfigsToPluginsConfig(matchers))
}

// DefaultPalantirConfigMergedWithExcludeMatchersAndPluginConfig returns a GolangCILintConfig that is the result of
// merging the default Palantir golangci-lint configuration with the provided matchers and the provided plugin
// configuration.
func DefaultPalantirConfigMergedWithExcludeMatchersAndPluginConfig(baseConfig []byte, matchers matcher.NamesPathsCfg, config *PluginConfig) (GolangCILintConfig, error) {
	defaultConfig, err := MergeExcludeMatchersWithConfig(baseConfig, matchers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create default Palantir config with exclude matchers")
	}

	mergedConfig, err := MergePluginConfigWithConfig(defaultConfig, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to merge plugin config with default Palantir config")
	}

	return mergedConfig, nil
}

func convertNamesPathConfigsToPluginsConfig(namesPathsCfg matcher.NamesPathsCfg) *PluginConfig {
	if len(namesPathsCfg.Names) == 0 && len(namesPathsCfg.Paths) == 0 {
		return nil
	}
	return &PluginConfig{
		Linters: LintersConfig{
			Exclusions: ExclusionsConfig{
				Paths: convertNamesPathConfigsToExclusionsPaths(namesPathsCfg),
			},
		},
	}
}

func convertNamesPathConfigsToExclusionsPaths(namesPathsCfg matcher.NamesPathsCfg) []string {
	if len(namesPathsCfg.Names) == 0 && len(namesPathsCfg.Paths) == 0 {
		return nil
	}

	out := make([]string, 0, len(namesPathsCfg.Names)*2+len(namesPathsCfg.Paths)*2)
	for _, name := range namesPathsCfg.Names {
		// name within a directory
		out = append(out, fmt.Sprintf(`.+/%s$`, name))

		// top-level name
		out = append(out, fmt.Sprintf(`^%s$`, name))
	}

	for _, path := range namesPathsCfg.Paths {
		// path prefix
		out = append(out, fmt.Sprintf(`%s/.*`, path))

		// path exact match
		out = append(out, fmt.Sprintf("^%s$", path))
	}
	return out
}

func MergePluginConfigWithConfig(configBytes GolangCILintConfig, cfg *PluginConfig) (GolangCILintConfig, error) {
	// if "version" is not set, set it to version 2.
	// This is explicitly required by golangci-lint per https://golangci-lint.run/docs/configuration/file/#version-configuration.
	if exists, err := checkNodeExists(configBytes, "/version"); err != nil {
		return nil, errors.Wrapf(err, "failed to check if version exists in config")
	} else if !exists {
		configBytes, err = goccyyamlpatcher.New().Apply(configBytes, yamlpatch.Patch{
			{
				Type:  yamlpatch.OperationAdd,
				Path:  yamlpatch.MustParsePath("/version"),
				Value: "2",
			},
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to add version to config")
		}
	}

	if cfg == nil {
		return configBytes, nil
	}

	var err error
	applied := configBytes

	applied, err = applyAddYAMLSlicePatch(applied, "/linters/enable", cfg.Linters.Enable)
	if err != nil {
		return nil, err
	}

	applied, err = applyAddYAMLSlicePatch(applied, "/linters/disable", cfg.Linters.Disable)
	if err != nil {
		return nil, err
	}

	applied, err = applyAddOrSetYAMLMapPatch(applied, "/linters/settings", cfg.Linters.Settings)
	if err != nil {
		return nil, err
	}

	applied, err = applyAddYAMLSlicePatch(applied, "/linters/exclusions/rules", cfg.Linters.Exclusions.Rules)
	if err != nil {
		return nil, err
	}

	applied, err = applyAddYAMLSlicePatch(applied, "/linters/exclusions/paths", cfg.Linters.Exclusions.Paths)
	if err != nil {
		return nil, err
	}

	applied, err = applyAddYAMLSlicePatch(applied, "/linters/exclusions/paths-except", cfg.Linters.Exclusions.PathsExcept)
	if err != nil {
		return nil, err
	}

	return applied, nil
}

func applyAddOrSetYAMLMapPatch(yamlBytes []byte, yamlPath string, mapValue yaml.MapSlice) ([]byte, error) {
	if len(mapValue) == 0 {
		// if map is empty, no patch to apply
		return yamlBytes, nil
	}

	patcher := goccyyamlpatcher.New()
	applied := yamlBytes

	addOrSetValuePatch, err := createSetYAMLMapValuePatch(applied, yamlPath, mapValue)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create patch for %s", yamlPath)
	}
	applied, err = patcher.Apply(applied, addOrSetValuePatch)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for %s", yamlPath)
	}
	return applied, nil
}

func applyAddYAMLSlicePatch[T any](yamlBytes []byte, yamlPath string, slice []T) ([]byte, error) {
	if len(slice) == 0 {
		// if the slice is empty, no patch to apply
		return yamlBytes, nil
	}

	patcher := goccyyamlpatcher.New()
	applied := yamlBytes

	addSlicePatch, err := createAddYAMLSlicePatch(applied, yamlPath, slice)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create patch for %s", yamlPath)
	}
	applied, err = patcher.Apply(applied, addSlicePatch)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for %s", yamlPath)
	}
	return applied, nil
}

func createObjectsToAddForPath(yamlPath string, value any) []any {
	var result []any
	segments := strings.Split(yamlPath, "/")
	for idx := len(segments) - 1; idx >= 0; idx-- {
		if idx == len(segments)-1 {
			// for the last segment, we just append the value
			result = append(result, value)
		} else {
			// otherwise, current value is a map with key as segment and value as the previous result
			result = append([]any{map[string]any{segments[idx+1]: result[0]}}, result...)
		}
	}
	return result
}

func addOrSetYAMLPatch(yamlBytes []byte, yamlPath string, setYAMLPatch yamlpatch.Patch, value any) (yamlpatch.Patch, error) {
	parsedPath, err := yamlpatch.ParsePath(yamlPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse yamlPath %q", yamlPath)
	}

	if len(parsedPath) <= 1 {
		return nil, errors.Errorf("YAML path must have at least one segment after the root, but was %q", yamlPath)
	}

	nodeExists, err := checkNodeExists(yamlBytes, yamlPath)
	if err != nil {
		return nil, err
	}

	if nodeExists {
		// if destination node exists, return "set" patch
		return setYAMLPatch, nil
	}

	// creates a map for the full path
	objectsToAdd := createObjectsToAddForPath(yamlPath, value)

	// walk path and insert corresponding value at furthest existing path
	pathSoFar := ""
	furthestExistingIdx := -1
	for idx, part := range parsedPath {
		// special case: the path to a node is constructed as a series of "/<part>" segments, and the root node is
		// represented by "/" (conceptually, "/<part>", where <part> is ""). However, for the first child node "child",
		// the path is simply "/child", not "//child" (as it should be if the general rule was followed). In the general
		// case, every element adds a "/<part>" to the path. However, for the second element (idx == 1), adding
		// "/<part>" would result in the path being "//<part>", which is incorrect. As such, for idx == 1 only,
		// pathSoFar is reset to an empty string so that adding "/<part>" results in the correct path. This approach is
		// taken so that the path for idx == 0 is "/" and so that subsequent paths can be constructed correctly using
		// the general logic.
		if idx == 1 {
			pathSoFar = ""
		}
		pathSoFar += "/" + part

		nodeExists, err := checkNodeExists(yamlBytes, pathSoFar)
		if err != nil {
			return nil, err
		}
		if nodeExists {
			// if node exists, check next level
			continue
		}
		furthestExistingIdx = idx
		break
	}
	return yamlpatch.Patch{
		{
			Type:  yamlpatch.OperationAdd,
			Path:  yamlpatch.MustParsePath(pathSoFar),
			Value: objectsToAdd[furthestExistingIdx],
		},
	}, nil
}

// Creates a YAML patch that sets the value at the specified path in the YAML document to the provided map value. If the
// path to the node exists, the items in the provided map are added to the existing map. If the path up to the node does
// not exist, returns a patch that creates the node up to the node, assuming that key of the keys to the node is a map.
func createSetYAMLMapValuePatch(yamlBytes []byte, yamlPath string, mapValue yaml.MapSlice) (yamlpatch.Patch, error) {
	if len(mapValue) == 0 {
		return nil, nil
	}
	var setPatch yamlpatch.Patch
	for idx, mapItem := range mapValue {
		mapItemKey, ok := mapItem.Key.(string)
		if !ok {
			return nil, errors.Errorf("map key %v at index %d is not a string", mapItem.Key, idx)
		}

		yamlPathToMapItem := yamlPath + "/" + mapItemKey

		nodeExists, err := checkNodeExists(yamlBytes, yamlPathToMapItem)
		if err != nil {
			return nil, err
		}

		opType := yamlpatch.OperationAdd
		if nodeExists {
			opType = yamlpatch.OperationReplace
		}
		setPatch = append(setPatch, yamlpatch.Operation{
			Type:  opType,
			Path:  yamlpatch.MustParsePath(yamlPathToMapItem),
			Value: mapItem.Value,
		})
	}
	return addOrSetYAMLPatch(yamlBytes, yamlPath, setPatch, mapValue)
}

// Creates a YAML patch that adds a slice of items to a specified path in the YAML document. Supports both adding to an
// existing node or creating a new node if it does not exist. If the path up to the node does not exist, returns a patch
// that creates the node up to the node, assuming that key of the keys to the node is a map.
func createAddYAMLSlicePatch[T any](yamlBytes []byte, yamlPath string, slice []T) (yamlpatch.Patch, error) {
	if len(slice) == 0 {
		return nil, nil
	}
	var setPatch yamlpatch.Patch
	for _, linter := range slice {
		setPatch = append(setPatch, yamlpatch.Operation{
			Type:  yamlpatch.OperationAdd,
			Path:  yamlpatch.MustParsePath(yamlPath + "/-"),
			Value: linter,
		})
	}
	return addOrSetYAMLPatch(yamlBytes, yamlPath, setPatch, slice)
}

// Returns true if the node at the specified YAML path exists in the provided YAML bytes, false otherwise. The provided
// YAML path should be in the format used by YAML patch, e.g. "/path/to/node".
func checkNodeExists(yamlBytes []byte, yamlPath string) (bool, error) {
	yPath, err := yaml.PathString(yamlPatchPathToGoccyPathString(yamlPath))
	if err != nil {
		return false, fmt.Errorf("failed to parse yamlPath %q: %w", yamlPath, err)
	}

	node, err := yPath.ReadNode(bytes.NewReader(yamlBytes))
	if err != nil {
		if errors.Is(err, yaml.ErrNotFoundNode) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read node from YAML bytes: %w", err)
	}
	return node != nil, nil
}

// Converts a YAML patch path (e.g. "/path/to/node") to a goccy/go-yaml compatible path string (e.g. "$.path.to.node").
// Implemented as a simple find-replace that converts '/' to '.' and prepends a '$' to the string -- this is a simple
// implementation that may not necessarily handle every valid input.
func yamlPatchPathToGoccyPathString(in string) string {
	goccyPath := "$"
	if in != "/" {
		goccyPath += strings.ReplaceAll(in, "/", ".")
	}
	return goccyPath
}
