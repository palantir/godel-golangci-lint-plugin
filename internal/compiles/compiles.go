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

package compiles

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

// Run loads the packages matching the provided patterns and prints any loading errors to the provided writer. Returns
// exit code 0 if there were no errors, or 1 if there were any errors.
//
// If any package-level errors are encountered, they are printed to the provided writer and a summary is printed at the
// end. If no errors are encountered, nothing is printed to the provided writer (unless debugMode is true).
//
// If debugMode is true, prints debug information about loading time to the provided writer.
//
// When used before invoking golangci-lint, the packages.Config that is used to load packages by this function should
// semantically match the packages.Config that will be used by golangci-lint. Currently, this invariant is known to be
// true based on the design: godel-golangci-lint-plugin does not expose any way for end users to specify build tags or
// exclude test packages, so the configuration passed to packages.Load by this function should match the configuration
// used by golangci-lint. However, if aspects like these are made configurable in the future, this function needs to be
// updated to ensure that it loads packages in the same manner (otherwise, the errors reported by this function may be
// for packages/load modes that do not match those used by golangci-lint).
func Run(pkgPatterns []string, debugMode bool, out io.Writer) int {
	var startLoadingTime time.Time
	if debugMode {
		_, _ = fmt.Fprintln(out, "Loading packages for \"compiles\" check:", strings.Join(pkgPatterns, " "))
		startLoadingTime = time.Now()
	}
	initialPkgs, err := load(pkgPatterns)
	if err != nil {
		_, _ = fmt.Fprintln(out, err)
		return 1
	}
	if debugMode {
		timeToLoadPackages := time.Since(startLoadingTime)
		_, _ = fmt.Fprintln(out, "Finished loading packages for \"compiles\" check: took", timeToLoadPackages.String())
	}

	if numIssues := printErrors(initialPkgs, out, modulePathPkgErrorProcessor); numIssues > 0 {
		_, _ = fmt.Fprintf(out, "%d issues:\n", numIssues)
		_, _ = fmt.Fprintf(out, "* compiles: %d\n", numIssues)
		return 1
	}

	return 0
}

func SuccessOutput(out io.Writer) {
	// match output of golangci-lint when no issues are found
	_, _ = fmt.Fprintln(out, "0 issues.")
}

// load loads the specified packages. Returns only top-level loading errors. Does not consider errors in packages.
// Loads packages using the mode (packages.LoadAllSyntax | packages.NeedModule) and with tests included. Does not
// configure any custom build flags or environment variables.
func load(patterns []string) ([]*packages.Package, error) {
	conf := packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Tests: true,
	}
	initial, err := packages.Load(&conf, patterns...)
	if err == nil && len(initial) == 0 {
		err = fmt.Errorf("%s matched no packages", strings.Join(patterns, " "))
	}
	return initial, err
}

type pkgErrorProcessor func(pkg *packages.Package, err packages.Error) packages.Error

// modulePathPkgErrorProcessor is a pkgErrorProcessor that trims the module path prefix from the position of each error.
func modulePathPkgErrorProcessor(pkg *packages.Package, err packages.Error) packages.Error {
	if pkg.Module == nil {
		return err
	}
	prefixToTrim := pkg.Module.Dir
	const pathSeparator = string(filepath.Separator)
	if prefixToTrim != "" && !strings.HasSuffix(prefixToTrim, pathSeparator) {
		prefixToTrim += pathSeparator
	}
	err.Pos = strings.TrimPrefix(err.Pos, prefixToTrim)
	return err
}

// printErrors prints all errors in the provided packages to the provided writer, one per line. If there is a module
// error in the package, prints it once per package. Packages are visited according to the order returned by
// packages.Postorder. If processPkgError is not nil, it is called for each package error and its result is printed.
//
// Based on the https://pkg.go.dev/golang.org/x/tools/go/packages#PrintErrors function -- the primary differences are
// that output is printed to the supplied io.Writer (rather than hard-coding os.Stderr), and that package errors can be
// processed by the provided function before being printed.
func printErrors(pkgs []*packages.Package, out io.Writer, processPkgError pkgErrorProcessor) int {
	var n int
	errModules := make(map[*packages.Module]bool)
	seenErrs := make(map[packages.Error]bool)
	for pkg := range packages.Postorder(pkgs) {
		// print all package errors
		for _, err := range pkg.Errors {
			if processPkgError != nil {
				err = processPkgError(pkg, err)
			}
			// skip duplicate errors (common case is regular package and test package both reporting the same error)
			if seenErrs[err] {
				continue
			}
			_, _ = fmt.Fprintln(out, err.Error())
			n++
			seenErrs[err] = true
		}

		// print pkg.Module.Error once if present
		mod := pkg.Module
		if mod != nil && mod.Error != nil && !errModules[mod] {
			errModules[mod] = true
			_, _ = fmt.Fprintln(out, mod.Error.Err)
			n++
		}
	}
	return n
}
