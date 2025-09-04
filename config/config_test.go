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
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/palantir/pkg/matcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultPalantirConfigContent = `version: "2"

linters:
  default: none

  settings:
    custom:
      # Enable the custom "compiles" linter
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.

  # Enable Palantir-specific linters
  enable:
    - compiles
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused

run:
  relative-path-mode: gomod
`

func Test_addPluginConfigToYAML(t *testing.T) {
	for i, tc := range []struct {
		name         string
		baseConfig   string
		pluginConfig string
		want         string
	}{
		{
			name: "adds enable element to base config that has no linters element",
			baseConfig: `version: "2"
`,
			pluginConfig: `linters:
  # additive to default
  enable:
    - copyloopvar
`,
			want: `version: "2"
linters:
  enable:
    - copyloopvar
`,
		},
		{
			name: "adds enable element to base config that has no linters.enable element",
			baseConfig: `version: "2"
linters:
  default: none
`,
			pluginConfig: `linters:
  # additive to default
  enable:
    - copyloopvar
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - copyloopvar
`,
		},
		{
			name: "adds enable element to base config that has a linters.enable element",
			baseConfig: `version: "2"
linters:
  default: none
  enable:
    - compiles
`,
			pluginConfig: `linters:
  # additive to default
  enable:
    - copyloopvar
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - compiles
    - copyloopvar
`,
		},
		{
			name: "adds disable element to base config that has a linters.disable element",
			baseConfig: `version: "2"
linters:
  default: none
  enable:
    - compiles
`,
			pluginConfig: `linters:
  # additive to default
  disable:
    - copyloopvar
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - compiles
  disable:
    - copyloopvar
`,
		},
		{
			name: "adds linters.exclusions.rules elements to base config that has a linters element",
			baseConfig: `version: "2"
linters:
  default: none
  enable:
    - compiles
`,
			pluginConfig: `linters:
  exclusions:
    rules:
      - linters:
          - revive
        text: "should have comment or be unexported"
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - compiles
  exclusions:
    rules:
      - linters:
          - revive
        text: should have comment or be unexported
`,
		},
		{
			name: "adds linters.exclusions.rules elements to base config that has a linters.exclusions.rules element",
			baseConfig: `version: "2"
linters:
  default: none
  enable:
    - compiles
  exclusions:
    rules:
      - linters:
          - compiles
        text: test text for compiles
`,
			pluginConfig: `linters:
  exclusions:
    rules:
      - linters:
          - revive
        text: "should have comment or be unexported"
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - compiles
  exclusions:
    rules:
      - linters:
          - compiles
        text: test text for compiles
      - linters:
          - revive
        text: should have comment or be unexported
`,
		},
		{
			name: "adds linters.exclusions.paths elements to base config that has a linters element",
			baseConfig: `version: "2"
linters:
  default: none
  enable:
    - compiles
`,
			pluginConfig: `linters:
  exclusions:
    paths:
      - lib/bad.go
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - compiles
  exclusions:
    paths:
      - lib/bad.go
`,
		},
		{
			name: "adds linters.exclusions.paths elements to base config that has a linters.exclusions.paths element",
			baseConfig: `version: "2"
linters:
  default: none
  enable:
    - compiles
  exclusions:
    paths:
      - lib/original.go
`,
			pluginConfig: `linters:
  exclusions:
    paths:
      - lib/bad.go
`,
			want: `version: "2"
linters:
  default: none
  enable:
    - compiles
  exclusions:
    paths:
      - lib/original.go
      - lib/bad.go
`,
		},
		{
			name: "merges full configuration",
			baseConfig: `version: "2"

linters:
  default: none

  # Enable Palantir-specific linters
  enable:
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused

  exclusions:
    rules:
      - linters:
          - compiles
        text: test text for compiles
    paths:
      - lib/base.go

run:
  relative-path-mode: gomod
`,
			pluginConfig: `linters:
  enable:
    - copyloopvar
  disable:
    - asasalint
  exclusions:
    rules:
      - linters:
          - revive
        text: should have comment or be unexported
    paths:
      - lib/bad.go
`,
			want: `version: "2"

linters:
  default: none

  # Enable Palantir-specific linters
  enable:
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused
    - copyloopvar

  exclusions:
    rules:
      - linters:
          - compiles
        text: test text for compiles
      - linters:
          - revive
        text: should have comment or be unexported
    paths:
      - lib/base.go
      - lib/bad.go
  disable:
    - asasalint

run:
  relative-path-mode: gomod
`,
		},
	} {
		t.Run(fmt.Sprintf("Case %d: %s", i, tc.name), func(t *testing.T) {
			var cfg PluginConfig
			err := yaml.Unmarshal([]byte(tc.pluginConfig), &cfg)
			require.NoError(t, err, "failed to unmarshal test plugin config")

			applied, err := MergePluginConfigWithConfig(GolangCILintConfig(tc.baseConfig), &cfg)
			require.NoError(t, err)

			assert.Equal(t, tc.want, string(applied))
		})
	}
}

func Test_DefaultPalantirConfig(t *testing.T) {
	for i, tc := range []struct {
		name     string
		matchers matcher.NamesPathsCfg
		want     string
	}{
		{
			name: "default Palantir config converts matcher exclusions to linter exclusion paths",
			matchers: matcher.NamesPathsCfg{
				Names: []string{
					`.*\.conjure.go`,
				},
				Paths: []string{
					"internal/generated",
				},
			},
			want: `version: "2"

linters:
  default: none

  settings:
    custom:
      # Enable the custom "compiles" linter
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.

  # Enable Palantir-specific linters
  enable:
    - compiles
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused
  exclusions:
    paths:
      - ".+/.*\\.conjure.go$"
      - "^.*\\.conjure.go$"
      - internal/generated/.*
      - ^internal/generated$

run:
  relative-path-mode: gomod
`,
		},
		{
			name: "default Palantir config converts empty matcher exclusions to empty linter exclusion paths",
			matchers: matcher.NamesPathsCfg{
				Names: []string{},
				Paths: []string{},
			},
			want: `version: "2"

linters:
  default: none

  settings:
    custom:
      # Enable the custom "compiles" linter
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.

  # Enable Palantir-specific linters
  enable:
    - compiles
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused

run:
  relative-path-mode: gomod
`,
		},
	} {
		t.Run(fmt.Sprintf("Case %d: %s", i, tc.name), func(t *testing.T) {
			gotDefaultConfig, err := MergeExcludeMatchersWithConfig([]byte(defaultPalantirConfigContent), tc.matchers)
			require.NoError(t, err)

			assert.Equal(t, tc.want, string(gotDefaultConfig))
		})
	}
}

func Test_MergePluginConfigWithDefaultPalantirConfig(t *testing.T) {
	for i, tc := range []struct {
		name         string
		matchers     matcher.NamesPathsCfg
		pluginConfig string
		want         string
	}{
		{
			name:         "empty plugin config merges with default Palantir config",
			pluginConfig: "",
			want: `version: "2"

linters:
  default: none

  settings:
    custom:
      # Enable the custom "compiles" linter
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.

  # Enable Palantir-specific linters
  enable:
    - compiles
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused

run:
  relative-path-mode: gomod
`,
		},
		{
			name: "empty plugin config merges with default Palantir config with matchers",
			matchers: matcher.NamesPathsCfg{
				Names: []string{
					`.*\.conjure.go`,
				},
				Paths: []string{
					"internal/generated",
				},
			},
			pluginConfig: "",
			want: `version: "2"

linters:
  default: none

  settings:
    custom:
      # Enable the custom "compiles" linter
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.

  # Enable Palantir-specific linters
  enable:
    - compiles
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused
  exclusions:
    paths:
      - ".+/.*\\.conjure.go$"
      - "^.*\\.conjure.go$"
      - internal/generated/.*
      - ^internal/generated$

run:
  relative-path-mode: gomod
`,
		},
		{
			name: "full plugin config merges with default Palantir config",
			pluginConfig: `
linters:
  enable:
    - copyloopvar
  disable:
    - compiles
  settings:
    errcheck:
      check-type-assertions: true
      check-blank: true
  exclusions:
    rules:
      - path: _test\.go
        linters:
          - errcheck
    paths:
      - ".*\\.my\\.go$"
    paths-except:
      - lib/bad.go
`,
			want: `version: "2"

linters:
  default: none

  settings:
    custom:
      # Enable the custom "compiles" linter
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.
    errcheck:
      check-blank: true
      check-type-assertions: true

  # Enable Palantir-specific linters
  enable:
    - compiles
    - errcheck
    - govet
    - ineffassign
    - revive
    - unconvert
    - unused
    - copyloopvar
  disable:
    - compiles
  exclusions:
    rules:
      - linters:
          - errcheck
        path: "_test\\.go"
    paths:
      - ".*\\.my\\.go$"
    paths-except:
      - lib/bad.go

run:
  relative-path-mode: gomod
`,
		},
	} {
		t.Run(fmt.Sprintf("Case %d: %s", i, tc.name), func(t *testing.T) {
			gotDefaultConfig, err := MergeExcludeMatchersWithConfig([]byte(defaultPalantirConfigContent), tc.matchers)
			require.NoError(t, err)

			var config PluginConfig
			err = yaml.Unmarshal([]byte(tc.pluginConfig), &config)
			require.NoError(t, err)

			gotConfig, err := MergePluginConfigWithConfig(gotDefaultConfig, &config)
			require.NoError(t, err)

			assert.Equal(t, tc.want, string(gotConfig))
		})
	}
}

func Test_applyAddOrSetYAMLMapPatch(t *testing.T) {
	for i, tc := range []struct {
		name     string
		in       string
		path     string
		mapValue yaml.MapSlice
		want     string
	}{
		{
			name: "creates path to map",
			in:   ``,
			path: "/path/to/map",
			mapValue: yaml.MapSlice{
				{
					Key:   "key-1",
					Value: "value-1",
				},
				{
					Key:   "key-2",
					Value: 2,
				},
				{
					Key: "key-3",
					Value: map[string]string{
						"inner-key-1": "inner-value-1",
					},
				},
			},
			want: `path:
  to:
    map:
      key-1: value-1
      key-2: 2
      key-3:
        inner-key-1: inner-value-1
`,
		},
		{
			name: "adds entries to existing map at path",
			in: `linters:
  settings:
    custom:
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.
`,
			path: "/linters/settings",
			mapValue: yaml.MapSlice{
				{
					Key: "errcheck",
					Value: yaml.MapSlice{
						{
							Key:   "check-type-assertions",
							Value: true,
						},
						{
							Key: "exclude-functions",
							Value: []string{
								"io/ioutil.ReadFile",
								"io.Copy(*bytes.Buffer)",
							},
						},
					},
				},
			},
			want: `linters:
  settings:
    custom:
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.
    errcheck:
      check-type-assertions: true
      exclude-functions:
        - io/ioutil.ReadFile
        - io.Copy(*bytes.Buffer)
`,
		},
		{
			name: "overwrites entries with same key in existing map at path",
			in: `linters:
  settings:
    custom:
      compiles:
        type: "module"
        description: A linter that verifies that the code compiles successfully.
`,
			path: "/linters/settings/custom",
			mapValue: yaml.MapSlice{
				{
					Key: "compiles",
					Value: yaml.MapSlice{
						{
							Key:   "type",
							Value: "module",
						},
						{
							Key:   "description",
							Value: "Custom description",
						},
						{
							Key:   "new-key",
							Value: "new-value",
						},
					},
				},
			},
			want: `linters:
  settings:
    custom:
      compiles:
        type: module
        description: Custom description
        new-key: new-value
`,
		},
	} {
		t.Run(fmt.Sprintf("Case %d: %s", i, tc.name), func(t *testing.T) {
			got, err := applyAddOrSetYAMLMapPatch([]byte(tc.in), tc.path, tc.mapValue)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}
