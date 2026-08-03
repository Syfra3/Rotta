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
}

func NewDisplayedApprovalAction(featureID, contractFingerprint, lifecycleAction string) *DisplayedApprovalAction {
	return &DisplayedApprovalAction{
		featureID:           featureID,
		contractFingerprint: contractFingerprint,
		lifecycleAction:     lifecycleAction,
	}
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
