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

package runner

import (
	goerror "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func RunGolangCILintWithConfig(pathToBinary string, preConfigArgs, postConfigArgs []string, configContent []byte, stdout, stderr io.Writer, debugMode bool) (int, error) {
	configFilePath, err := writeTempFile("golangci-lint-plugin-config-*.yml", configContent)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to write config file")
	}
	defer func() {
		// in debug mode, do not remove the config file
		if debugMode {
			return
		}
		_ = os.Remove(configFilePath)
	}()

	args := append(preConfigArgs, "--config", configFilePath)
	args = append(args, postConfigArgs...)

	return RunGolangCILint(pathToBinary, args, stdout, stderr, debugMode), nil
}

func RunGolangCILint(pathToBinary string, args []string, stdout, stderr io.Writer, debugMode bool) int {
	runner := GolangCILintCmdRunner(pathToBinary, args, stdout, stderr, debugMode)
	return runner()
}

func GolangCILintCmdRunner(pathToBinary string, args []string, stdout, stderr io.Writer, debugMode bool) func() int {
	cmd := exec.Command(pathToBinary, args...)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return func() int {
		if debugMode {
			_, _ = fmt.Fprintf(stderr, "Running \"%s\" in working directory %s\n", strings.Join(cmd.Args, " "), cmd.Dir)
		}
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode()
			}
			_, _ = fmt.Fprintf(stderr, "command %v failed with error: %v\n", cmd.Args, errors.Wrapf(err, "run error"))
			return 1
		}
		return 0
	}
}

func writeTempFile(pattern string, content []byte) (tmpFilePath string, rErr error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temp file")
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			rErr = goerror.Join(rErr, errors.Wrapf(err, "failed to close temp file"))
		}
	}()
	if _, err := tmpFile.Write(content); err != nil {
		return "", errors.Wrapf(err, "failed to write to temp file")
	}
	return tmpFile.Name(), nil
}
