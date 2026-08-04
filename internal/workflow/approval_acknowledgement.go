package workflow

import (
	"fmt"
	"strings"
)

// DisplayedApprovalAction is the sole lifecycle action presented for a
// feature and its current contract fingerprint.
type DisplayedApprovalAction struct {
	featureID           string
	contractFingerprint string
	lifecycleAction     string
	consumed            bool
	context             ApprovalAcknowledgementContext
}

// ApprovalAcknowledgementContext identifies the active prompt and every
// fingerprint that must remain unchanged before its acknowledgement advances.
type ApprovalAcknowledgementContext struct {
	PromptID            string
	SessionID           string
	FeatureID           string
	ContractFingerprint string
	PolicyFingerprint   string
	FinalSnapshot       string
	PendingActions      int
}

// FinalHumanReview is the recorded final-review state for one feature. It
// contains no human identity because final acknowledgement is actor-less.
type FinalHumanReview struct {
	FeatureID                   string
	Phase                       string
	CurrentCommit               string
	ReviewedCommit              string
	CurrentEvidenceFingerprint  string
	ReviewedEvidenceFingerprint string
	CurrentPolicyFingerprint    string
	ReviewedPolicyFingerprint   string
	PendingActions              int
	Completed                   bool
}

// DisplayedFinalApprovalAction binds a final approval prompt to one feature
// and its reviewed commit.
type DisplayedFinalApprovalAction struct {
	featureID      string
	reviewedCommit string
}

func NewDisplayedApprovalAction(featureID, contractFingerprint, lifecycleAction string) *DisplayedApprovalAction {
	return &DisplayedApprovalAction{
		featureID:           featureID,
		contractFingerprint: contractFingerprint,
		lifecycleAction:     lifecycleAction,
	}
}

// NewContextualDisplayedApprovalAction binds one displayed action to the
// current prompt, session, feature, and fingerprinted workflow context.
func NewContextualDisplayedApprovalAction(context ApprovalAcknowledgementContext, lifecycleAction string) *DisplayedApprovalAction {
	return &DisplayedApprovalAction{
		featureID:           context.FeatureID,
		contractFingerprint: context.ContractFingerprint,
		lifecycleAction:     lifecycleAction,
		context:             context,
	}
}

func NewDisplayedFinalApprovalAction(featureID, reviewedCommit string) *DisplayedFinalApprovalAction {
	return &DisplayedFinalApprovalAction{featureID: featureID, reviewedCommit: reviewedCommit}
}

// Approve completes the final review represented by this displayed action.
func (action *DisplayedFinalApprovalAction) Approve(acknowledgement string, review *FinalHumanReview) error {
	if strings.TrimSpace(strings.ToLower(acknowledgement)) != "approve" {
		return fmt.Errorf("final approval requires the exact acknowledgement approve")
	}
	if review.Phase != "final_human_review" {
		return fmt.Errorf("final approval requires final_human_review")
	}
	if review.CurrentCommit != review.ReviewedCommit {
		return fmt.Errorf("final approval requires the current commit to match reviewed_commit")
	}
	if review.CurrentEvidenceFingerprint != review.ReviewedEvidenceFingerprint {
		return fmt.Errorf("final approval requires the current evidence fingerprint to match reviewed_commit")
	}
	if review.CurrentPolicyFingerprint != review.ReviewedPolicyFingerprint {
		return fmt.Errorf("final approval requires the current policy fingerprint to match reviewed_commit")
	}
	if action.featureID != review.FeatureID {
		return fmt.Errorf("displayed final approval action does not match its feature")
	}
	if action.reviewedCommit != review.ReviewedCommit {
		return fmt.Errorf("displayed final approval action does not match reviewed_commit")
	}
	if review.PendingActions != 1 {
		return fmt.Errorf("final approval requires exactly one displayed action")
	}
	review.Phase = "complete"
	review.Completed = true
	return nil
}

// ConsumeAcknowledgement advances only this displayed action once when the
// reply and its current feature and contract bindings match the display.
func (action *DisplayedApprovalAction) ConsumeAcknowledgement(acknowledgement, featureID, contractFingerprint string, advance func(string) error) error {
	if action.consumed {
		return fmt.Errorf("displayed approval action has already been consumed")
	}
	if action.featureID != featureID || action.contractFingerprint != contractFingerprint {
		return fmt.Errorf("displayed approval action does not match its feature or contract")
	}
	if !isCompactAcknowledgement(acknowledgement) {
		return fmt.Errorf("acknowledgement is not an exact approval token")
	}
	return action.advance(advance)
}

// ConsumeContextualAcknowledgement advances only an unambiguous, current
// displayed action. Rejections occur before the lifecycle callback is called.
func (action *DisplayedApprovalAction) ConsumeContextualAcknowledgement(acknowledgement string, current ApprovalAcknowledgementContext, advance func(string) error) error {
	if action.consumed {
		return fmt.Errorf("displayed approval action has already been consumed")
	}
	if action.context.PromptID != current.PromptID {
		return fmt.Errorf("acknowledgement prompt was replaced")
	}
	if action.context.SessionID != current.SessionID {
		return fmt.Errorf("acknowledgement session was restarted")
	}
	if action.context.FeatureID != current.FeatureID {
		return fmt.Errorf("displayed approval action does not match its feature")
	}
	if action.context.ContractFingerprint != current.ContractFingerprint {
		return fmt.Errorf("displayed approval action does not match its contract")
	}
	if action.context.PolicyFingerprint != current.PolicyFingerprint {
		return fmt.Errorf("displayed approval action does not match its policy")
	}
	if action.context.FinalSnapshot != current.FinalSnapshot {
		return fmt.Errorf("displayed approval action does not match its final snapshot")
	}
	if current.PendingActions != 1 {
		return fmt.Errorf("more than one approval action is pending")
	}
	if len(strings.Fields(strings.TrimSpace(acknowledgement))) > 1 {
		return fmt.Errorf("acknowledgement contains multiple intents")
	}
	if !isCompactAcknowledgement(acknowledgement) {
		return fmt.Errorf("acknowledgement is not an exact approval token")
	}
	return action.advance(advance)
}

func (action *DisplayedApprovalAction) advance(advance func(string) error) error {
	if err := advance(action.lifecycleAction); err != nil {
		return err
	}
	action.consumed = true
	return nil
}

func isCompactAcknowledgement(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x", "yes", "agree", "approved", "approve":
		return true
	default:
		return false
	}
}
