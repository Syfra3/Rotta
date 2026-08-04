package workflow

import (
	"fmt"
	"strings"
)

// FeatureReviewRename identifies the source and destination of one renamed
// path in the feature review range.
type FeatureReviewRename struct {
	From string
	To   string
}

// FeatureReviewChangedFiles is the baseline-to-HEAD file evidence used for a
// feature review after checkpoint commits have cleaned the worktree.
type FeatureReviewChangedFiles struct {
	BaselineSHA  string
	HEADSHA      string
	NameStatus   string
	ChangedPaths []string
	RenamedPaths []FeatureReviewRename
	DeletedPaths []string
}

// ResolveFeatureReviewChangedFiles derives review paths exclusively from the
// confirmed baseline through HEAD, rather than from the current worktree diff.
func ResolveFeatureReviewChangedFiles(repoRoot, baselineSHA string) (FeatureReviewChangedFiles, error) {
	baseline, err := gitSubmissionOutput(repoRoot, "rev-parse", baselineSHA+"^{commit}")
	if err != nil {
		return FeatureReviewChangedFiles{}, fmt.Errorf("resolve review baseline: %w", err)
	}
	head, err := gitSubmissionOutput(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return FeatureReviewChangedFiles{}, fmt.Errorf("resolve review HEAD: %w", err)
	}
	nameStatus, err := gitSubmissionOutput(repoRoot, "diff", "--name-status", baseline+"..."+head)
	if err != nil {
		return FeatureReviewChangedFiles{}, fmt.Errorf("resolve review changed files: %w", err)
	}

	resolved := FeatureReviewChangedFiles{BaselineSHA: baseline, HEADSHA: head, NameStatus: nameStatus}
	for _, line := range strings.Split(nameStatus, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		switch fields[0][0] {
		case 'R':
			if len(fields) == 3 {
				resolved.RenamedPaths = append(resolved.RenamedPaths, FeatureReviewRename{From: fields[1], To: fields[2]})
			}
		case 'D':
			resolved.DeletedPaths = append(resolved.DeletedPaths, fields[1])
		default:
			resolved.ChangedPaths = append(resolved.ChangedPaths, fields[1])
		}
	}
	return resolved, nil
}
