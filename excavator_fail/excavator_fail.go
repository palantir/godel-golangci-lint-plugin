package fail

fail

/*
This is a non-compiling file that has been added to explicitly ensure that CI fails.
It also contains the command that caused the failure and its output.
Remove this file if debugging locally.

go mod operation failed. This may mean that there are legitimate dependency issues with the "go.mod" definition in the repository and the updates performed by the gomod check. This branch can be cloned locally to debug the issue.

Command that caused error:
./godelw mod

Output:
go: finding module for package github.com/nunnatsa/ginkgolinter/types
go: github.com/golangci/golangci-lint/v2/cmd/golangci-lint imports
	github.com/golangci/golangci-lint/v2/pkg/commands imports
	github.com/golangci/golangci-lint/v2/pkg/lint/lintersdb imports
	github.com/golangci/golangci-lint/v2/pkg/golinters/ginkgolinter imports
	github.com/nunnatsa/ginkgolinter/types: module github.com/nunnatsa/ginkgolinter@latest found (v0.20.0), but does not contain package github.com/nunnatsa/ginkgolinter/types

*/
