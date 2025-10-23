package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runCollectNewServers identifies newly added local servers between two git
// revisions. It accepts --base, --head, --workspace, --output-json, and
// --summary-md flags, writing machine-readable targets and a Markdown summary
// for reviewers.
func runCollectNewServers(args []string) error {
	flags := flag.NewFlagSet("collect-new-servers", flag.ContinueOnError)
	base := flags.String("base", "", "base git commit SHA")
	head := flags.String("head", "", "head git commit SHA")
	workspace := flags.String("workspace", ".", "path to repository workspace")
	outputJSON := flags.String("output-json", "", "path to write JSON context")
	summaryMD := flags.String("summary-md", "", "path to write Markdown summary")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *base == "" || *head == "" || *outputJSON == "" || *summaryMD == "" {
		return errors.New("base, head, output-json, and summary-md are required")
	}

	targets, err := collectNewServerTargets(*workspace, *base, *head)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		removeIfPresent(*outputJSON)
		removeIfPresent(*summaryMD)
		return nil
	}

	if err := writeJSONFile(*outputJSON, targets); err != nil {
		return err
	}

	summary := buildNewServerSummary(targets)
	return os.WriteFile(*summaryMD, []byte(summary), 0o644)
}

// collectNewServerTargets returns metadata for local servers that were added
// between the supplied git revisions.
func collectNewServerTargets(workspace, base, head string) ([]newServerTarget, error) {
	lines, err := gitDiff(workspace, base, head, "--name-status")
	if err != nil {
		return nil, err
	}

	var targets []newServerTarget
	for _, line := range lines {
		if !strings.HasPrefix(line, "A\t") {
			continue
		}
		path := strings.TrimPrefix(line, "A\t")
		if !strings.HasPrefix(path, "servers/") || !strings.HasSuffix(path, "server.yaml") {
			continue
		}

		doc, err := loadServerYAMLFromWorkspace(workspace, path)
		if err != nil {
			continue
		}

		if !isLocalServer(doc) {
			continue
		}

		project := strings.TrimSpace(doc.Source.Project)
		commit := strings.TrimSpace(doc.Source.Commit)
		if project == "" || commit == "" {
			continue
		}

		targets = append(targets, newServerTarget{
			Server:    filepath.Base(filepath.Dir(path)),
			File:      path,
			Image:     strings.TrimSpace(doc.Image),
			Project:   project,
			Commit:    commit,
			Directory: strings.TrimSpace(doc.Source.Directory),
		})
	}

	return targets, nil
}

// buildNewServerSummary renders Markdown describing newly added servers for
// review prompts and human consumption.
func buildNewServerSummary(targets []newServerTarget) string {
	builder := strings.Builder{}
	builder.WriteString("## New Local Servers\n\n")

	for _, target := range targets {
		builder.WriteString(fmt.Sprintf("### %s\n", target.Server))
		builder.WriteString(fmt.Sprintf("- Repository: %s\n", target.Project))
		builder.WriteString(fmt.Sprintf("- Commit: `%s`\n", target.Commit))
		if target.Directory != "" {
			builder.WriteString(fmt.Sprintf("- Directory: %s\n", target.Directory))
		} else {
			builder.WriteString("- Directory: (repository root)\n")
		}
		builder.WriteString(fmt.Sprintf("- Checkout path: /tmp/security-review/new/%s/repo\n\n", target.Server))
	}

	return builder.String()
}
