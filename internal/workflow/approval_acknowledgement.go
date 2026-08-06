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

// FinalHumanReview is the recorded final-review state for one feature.
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

// DisplayedFinalApprovalAction is retained only to reject callers of the
// retired free-form final-approval path. Final completion requires a bound
// native Question decision.
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

// Approve rejects the retired free-form final-review completion path.
func (action *DisplayedFinalApprovalAction) Approve(acknowledgement string, review *FinalHumanReview) error {
	return fmt.Errorf("free-form final approval cannot complete review; use a bound native Question decision")
}

// ConsumeAcknowledgement advances only this displayed action once when the
// reply and its current feature and contract bindings match the display.
func (action *DisplayedApprovalAction) ConsumeAcknowledgement(acknowledgement, featureID, contractFingerprint string, advance func(string) error) error {
	return fmt.Errorf("legacy free-form approval acknowledgements cannot advance workflow state; use a bound native Question")
}

// ConsumeContextualAcknowledgement advances only an unambiguous, current
// displayed action. Rejections occur before the lifecycle callback is called.
func (action *DisplayedApprovalAction) ConsumeContextualAcknowledgement(acknowledgement string, current ApprovalAcknowledgementContext, advance func(string) error) error {
	return fmt.Errorf("legacy free-form approval acknowledgements cannot advance workflow state; use a bound native Question")
}

func (action *DisplayedApprovalAction) advance(advance func(string) error) error {
	if err := advance(action.lifecycleAction); err != nil {
		return err
	}
	action.consumed = true
	return nil
}

// isCompactAcknowledgement is retained for the separate feature-local
// override flow; it is not a final-review completion mechanism.
func isCompactAcknowledgement(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x", "yes", "agree", "approved", "approve":
		return true
	default:
		return false
	}
}
