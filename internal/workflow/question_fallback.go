package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// QuestionHost names the host adapter that tries to display a Rotta governance
// question. The decision semantics remain host-neutral; hosts only provide UI.
type QuestionHost string

const (
	QuestionHostOpenCode QuestionHost = "opencode"
	QuestionHostPi       QuestionHost = "pi"
	QuestionHostClaude   QuestionHost = "claude"
	QuestionHostText     QuestionHost = "text-fallback"
)

// HostQuestionAttempt records whether a host-native question path was used or
// the caller must render the bounded text fallback. It is evidence only; it is
// not approval.
type HostQuestionAttempt struct {
	Host             QuestionHost
	NativeAttempted  bool
	NativeSucceeded  bool
	FallbackRendered bool
	FailureReason    string
}

// ShouldRenderTextFallback returns true when a native host question was not
// available or did not produce a consumable answer. A fallback is still a
// bounded single-select prompt and must be consumed through the same displayed
// question identity checks.
func ShouldRenderTextFallback(attempt HostQuestionAttempt) bool {
	return attempt.Host != "" && (!attempt.NativeAttempted || !attempt.NativeSucceeded)
}

// RenderTextQuestionFallback renders a host-neutral single-select prompt for
// coding agents that cannot use a native question UI. Context is displayed so a
// human answer can be bound back to the current prompt, but the only acceptable
// replies remain listed options or unambiguous aliases.
func RenderTextQuestionFallback(question NativeQuestion) (string, error) {
	if err := validateNativeQuestion(question); err != nil {
		return "", err
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s\n\n", question.Header)
	fmt.Fprintf(&builder, "%s\n\n", question.Question)
	fmt.Fprintf(&builder, "Prompt ID: %s\n", question.Context.PromptID)
	fmt.Fprintf(&builder, "Workspace: %s\n", question.Context.Workspace)
	fmt.Fprintf(&builder, "Action: %s\n", question.Context.Action)
	fmt.Fprintf(&builder, "Reply with exactly one option. Examples: `1`, `option 1`, `1.`, or the exact option text. Suboption tokens such as `1a` are accepted only when the rendered option label begins with that token.\n\n")
	for index, option := range question.Options {
		fmt.Fprintf(&builder, "%d. %s — %s\n", index+1, option.Label, option.Description)
	}
	fmt.Fprintf(&builder, "\nAny custom, ambiguous, stale, or mismatched answer safely stops.\n")
	return builder.String(), nil
}

// ConsumeTextQuestionReply parses a bounded text fallback response and consumes
// it through the same displayed-question binding used by native host replies.
func ConsumeTextQuestionReply(displayed *DisplayedNativeQuestion, response string, current QuestionContext) (NativeQuestionResult, error) {
	if displayed == nil {
		return NativeQuestionResult{}, fmt.Errorf("text fallback question is not displayed")
	}
	label, err := displayed.textFallbackLabel(response)
	if err != nil {
		return NativeQuestionResult{}, err
	}
	return displayed.Consume(NativeQuestionAnswer{Context: displayed.question.Context, Label: label}, current)
}

// ConsumeExactStrictApprovalTextReply applies text fallback parsing to exact
// Strict approvals while preserving rendered-contract identity checks.
func ConsumeExactStrictApprovalTextReply(displayed *DisplayedNativeQuestion, response string, current ExactStrictApprovalContext) (NativeQuestionResult, error) {
	if displayed == nil || displayed.question.Approval == nil {
		return NativeQuestionResult{}, fmt.Errorf("exact Strict approval is not displayed")
	}
	if err := validateExactStrictApprovalContext(current); err != nil {
		return NativeQuestionResult{}, err
	}
	if *displayed.question.Approval != current {
		return NativeQuestionResult{}, fmt.Errorf("exact Strict approval binding does not match current rendered contract")
	}
	result, err := ConsumeTextQuestionReply(displayed, response, QuestionContext{PromptID: displayed.question.Context.PromptID, SessionID: current.SessionID, Workspace: current.Workspace, Action: current.Action})
	if err != nil {
		return NativeQuestionResult{}, err
	}
	result.ExactApproval = &ExactStrictApprovalEvidence{Context: current, Selected: result.Selected}
	return result, nil
}

// ConsumeExactOperationConsentTextReply applies text fallback parsing to an
// exact operation-consent prompt. It can create only the same unexecuted pending
// authorization record as the native path.
func ConsumeExactOperationConsentTextReply(displayed *DisplayedNativeQuestion, response string, current ExactOperationConsentContext) (NativeQuestionResult, error) {
	if displayed == nil {
		return NativeQuestionResult{}, fmt.Errorf("operation consent is not displayed")
	}
	if err := validateExactOperationConsentContext(current); err != nil {
		return NativeQuestionResult{}, err
	}
	if displayed.question.Consent == nil || *displayed.question.Consent != current || displayed.question.Context.Action != current.Action || displayed.question.Context.SessionID != current.SessionID || displayed.question.Context.Workspace != current.Workspace {
		return NativeQuestionResult{}, fmt.Errorf("operation consent binding does not match current rendered operation")
	}
	result, err := ConsumeTextQuestionReply(displayed, response, displayed.question.Context)
	if err != nil {
		return NativeQuestionResult{}, err
	}
	if result.Selected == ExactOperationConsentOption {
		copy := current
		result.PendingAction = &PendingExplicitAction{Kind: current.Action, ProjectRoot: current.Workspace, RequiresExplicitAuthorization: true, Consent: &copy}
	}
	return result, nil
}

func (question *DisplayedNativeQuestion) textFallbackLabel(response string) (string, error) {
	if question.closed {
		return "", fmt.Errorf("text fallback question is closed")
	}
	if question.replaced {
		return "", fmt.Errorf("text fallback question was replaced")
	}
	if err := question.validateIssuedQuestion(); err != nil {
		return "", err
	}
	input := normalizeTextFallbackAnswer(response)
	if input == "" {
		return "", fmt.Errorf("text fallback answer is empty")
	}

	matches := make([]string, 0, 1)
	for index, option := range question.question.Options {
		if textFallbackMatchesOption(input, index+1, option.Label) {
			matches = append(matches, option.Label)
		}
	}
	if len(matches) != 1 {
		if len(matches) == 0 {
			return "", fmt.Errorf("text fallback answer is not a current option")
		}
		return "", fmt.Errorf("text fallback answer is ambiguous")
	}
	return matches[0], nil
}

var optionPrefixPattern = regexp.MustCompile(`^option\s+`)

func normalizeTextFallbackAnswer(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	trimmed = strings.Trim(trimmed, "`\"'")
	trimmed = optionPrefixPattern.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimSuffix(trimmed, ".")
	trimmed = strings.TrimSuffix(trimmed, ")")
	trimmed = strings.TrimPrefix(trimmed, "(")
	return strings.Join(strings.Fields(trimmed), " ")
}

func textFallbackMatchesOption(input string, ordinal int, label string) bool {
	if input == normalizeTextFallbackAnswer(label) {
		return true
	}
	ordinalText := strconv.Itoa(ordinal)
	if input == ordinalText {
		return true
	}
	// Suboption aliases are intentionally conservative: `1a` is accepted only
	// when the visible option label itself starts with `1a`, so free-form custom
	// text cannot smuggle a new branch into a simple numeric prompt.
	if strings.HasPrefix(input, ordinalText) && len(input) > len(ordinalText) {
		labelToken := firstTextFallbackToken(label)
		return labelToken == input
	}
	return false
}

func firstTextFallbackToken(value string) string {
	fields := strings.Fields(normalizeTextFallbackAnswer(value))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
