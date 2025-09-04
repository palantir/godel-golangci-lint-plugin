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
	"github.com/palantir/godel/v2/framework/pluginapi/v2/pluginapi"
	"github.com/palantir/godel/v2/framework/verifyorder"
)

var (
	Version    = "unspecified"
	PluginInfo = pluginapi.MustNewPluginInfo(
		"com.palantir.godel-golangci-lint-plugin",
		"golangci-lint-plugin",
		Version,
		pluginapi.PluginInfoUsesConfigFile(),
		pluginapi.PluginInfoGlobalFlagOptions(
			pluginapi.GlobalFlagOptionsParamDebugFlag("--"+pluginapi.DebugFlagName),
			pluginapi.GlobalFlagOptionsParamProjectDirFlag("--"+pluginapi.ProjectDirFlagName),
			pluginapi.GlobalFlagOptionsParamGodelConfigFlag("--"+pluginapi.GodelConfigFlagName),
			pluginapi.GlobalFlagOptionsParamConfigFlag("--"+pluginapi.ConfigFlagName),
		),
		pluginapi.PluginInfoTaskInfo(
			lintCmd.Name(),
			lintCmd.Short,
			pluginapi.TaskInfoCommand(lintCmd.Name()),
			pluginapi.TaskInfoVerifyOptions(
				pluginapi.VerifyOptionsOrdering(intPtr(verifyorder.Check)),
				pluginapi.VerifyOptionsApplyTrueArgs("--fix"),
			),
		),
		pluginapi.PluginInfoTaskInfo(
			lintersCmd.Name(),
			lintersCmd.Short,
			pluginapi.TaskInfoCommand(lintersCmd.Name()),
		),
	)
)

func intPtr(val int) *int {
	return &val
}
