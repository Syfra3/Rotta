package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultChatSummaryLines = 80
	defaultChatSummaryBytes = 8 * 1024
)

// LifecycleCommandRequest identifies a lifecycle-relevant command whose full
// result must be persisted before it can be reported to chat.
type LifecycleCommandRequest struct {
	FeatureWorktree  string
	Command          []string
	WorkingDirectory string
	Timeout          time.Duration
	HostMaxLines     int
	HostMaxBytes     int
	CommandMetadata  json.RawMessage
}

// LifecycleCommandReport separates durable evidence from a bounded chat
// summary. Passing is true only after durable evidence was written.
type LifecycleCommandReport struct {
	EvidencePath string
	EvidenceHash string
	ChatSummary  string
	Passing      bool
}

type lifecycleCommandEvidence struct {
	Command          []string        `json:"command"`
	WorkingDirectory string          `json:"working_directory"`
	StartedAt        time.Time       `json:"started_at"`
	FinishedAt       time.Time       `json:"finished_at"`
	Stdout           string          `json:"stdout"`
	Stderr           string          `json:"stderr"`
	ExitStatus       int             `json:"exit_status"`
	TimedOut         bool            `json:"timed_out"`
	ContentHash      string          `json:"content_hash"`
	CommandMetadata  json.RawMessage `json:"command_metadata,omitempty"`
}

// CaptureLifecycleCommand runs the supplied command, persists its complete
// result in the feature worktree, then produces a bounded chat summary.
func CaptureLifecycleCommand(ctx context.Context, request LifecycleCommandRequest) (LifecycleCommandReport, error) {
	if request.FeatureWorktree == "" || request.WorkingDirectory == "" || len(request.Command) == 0 {
		return LifecycleCommandReport{}, fmt.Errorf("lifecycle command requires a feature worktree, working directory, and command")
	}
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	command := exec.CommandContext(ctx, request.Command[0], request.Command[1:]...)
	command.Dir = request.WorkingDirectory
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	startedAt := time.Now().UTC()
	runErr := command.Run()
	finishedAt := time.Now().UTC()

	exitStatus := 0
	if runErr != nil {
		exitStatus = -1
		if exitError, ok := runErr.(*exec.ExitError); ok {
			exitStatus = exitError.ExitCode()
		}
	}
	timedOut := ctx.Err() == context.DeadlineExceeded
	contentHash := commandOutputHash(stdout.String(), stderr.String())
	evidence := lifecycleCommandEvidence{
		Command:          append([]string(nil), request.Command...),
		WorkingDirectory: request.WorkingDirectory,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		Stdout:           stdout.String(),
		Stderr:           stderr.String(),
		ExitStatus:       exitStatus,
		TimedOut:         timedOut,
		ContentHash:      contentHash,
		CommandMetadata:  append(json.RawMessage(nil), request.CommandMetadata...),
	}
	evidencePath, err := writeLifecycleCommandEvidence(request.FeatureWorktree, evidence)
	if err != nil {
		return LifecycleCommandReport{}, fmt.Errorf("persist lifecycle command evidence: %w", err)
	}

	report := LifecycleCommandReport{
		EvidencePath: evidencePath,
		EvidenceHash: contentHash,
		Passing:      runErr == nil && !timedOut,
	}
	report.ChatSummary = boundedCommandSummary(evidence, evidencePath, commandSummaryLimits(request))
	return report, nil
}

func commandOutputHash(stdout, stderr string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(stdout))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(stderr))
	return hex.EncodeToString(hash.Sum(nil))
}

func writeLifecycleCommandEvidence(featureWorktree string, evidence lifecycleCommandEvidence) (string, error) {
	directory := filepath.Join(featureWorktree, ".rotta", "current", "evidence")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	contents, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(directory, fmt.Sprintf("command-%d-%s.json", evidence.StartedAt.UnixNano(), evidence.ContentHash[:12]))
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

type commandSummaryLimit struct {
	lines int
	bytes int
}

func commandSummaryLimits(request LifecycleCommandRequest) commandSummaryLimit {
	limits := commandSummaryLimit{lines: defaultChatSummaryLines, bytes: defaultChatSummaryBytes}
	if request.HostMaxLines > 0 && request.HostMaxLines < limits.lines {
		limits.lines = request.HostMaxLines
	}
	if request.HostMaxBytes > 0 && request.HostMaxBytes < limits.bytes {
		limits.bytes = request.HostMaxBytes
	}
	return limits
}

func boundedCommandSummary(evidence lifecycleCommandEvidence, evidencePath string, limits commandSummaryLimit) string {
	totalOutput := evidence.Stdout + evidence.Stderr
	totalLines := outputLines(totalOutput)
	truncated := totalLines > limits.lines || len(totalOutput) > limits.bytes
	failureExcerpt := "none"
	if evidence.ExitStatus != 0 || evidence.TimedOut {
		failureExcerpt = truncateSummaryValue(firstNonEmpty(evidence.Stderr, evidence.Stdout), 512)
	}
	summary := fmt.Sprintf("truncated: %t total_lines: %d total_bytes: %d exit_status: %d timed_out: %t failure_excerpt: %s evidence_path: %s evidence_hash: %s",
		truncated, totalLines, len(totalOutput), evidence.ExitStatus, evidence.TimedOut, failureExcerpt, evidencePath, evidence.ContentHash)
	return boundSummary(summary, limits)
}

func outputLines(output string) int {
	if output == "" {
		return 0
	}
	return strings.Count(output, "\n") + boolToInt(!strings.HasSuffix(output, "\n"))
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func truncateSummaryValue(value string, maxBytes int) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
	if len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}

func boundSummary(summary string, limits commandSummaryLimit) string {
	lines := strings.Split(summary, "\n")
	if len(lines) > limits.lines {
		lines = lines[:limits.lines]
	}
	summary = strings.Join(lines, "\n")
	if len(summary) > limits.bytes {
		return summary[:limits.bytes]
	}
	return summary
}
