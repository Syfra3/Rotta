package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type openCodeVelaTransaction struct {
	backupDir  string
	configPath string
	baseline   []byte
}

type openCodeVelaRollbackResult struct {
	Restored               bool `json:"restored"`
	ConcurrentModification bool `json:"concurrent_modification"`
}

type openCodeVelaRollbackEvidence struct {
	openCodeVelaRollbackResult
	Error string `json:"error,omitempty"`
}

func beginOpenCodeVelaTransaction(backupDir, configPath string) (*openCodeVelaTransaction, error) {
	baseline, err := readPrivateFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("capture Vela scoped transaction: %w", err)
	}
	return &openCodeVelaTransaction{backupDir: backupDir, configPath: configPath, baseline: baseline}, nil
}

func (transaction *openCodeVelaTransaction) recover(setupErr error) (openCodeVelaRollbackResult, error) {
	result := openCodeVelaRollbackResult{}
	current, err := readPrivateFile(transaction.configPath)
	if err != nil {
		return result, transaction.record(result, fmt.Errorf("inspect Vela scoped restore: %w", err))
	}
	if !onlyVelaManagedOpenCodeChange(transaction.baseline, current) {
		result.ConcurrentModification = true
		return result, transaction.record(result, fmt.Errorf("Vela configuration failed: %w; concurrent modification prevents scoped restore", setupErr))
	}
	if err := writePrivateFile(transaction.configPath, transaction.baseline, 0o600); err != nil {
		return result, transaction.record(result, fmt.Errorf("restore Vela scoped change: %w", err))
	}
	result.Restored = true
	return result, transaction.record(result, nil)
}

func onlyVelaManagedOpenCodeChange(baseline, current []byte) bool {
	if bytes.Equal(baseline, current) {
		return true
	}
	var before, after map[string]interface{}
	if json.Unmarshal(baseline, &before) != nil || json.Unmarshal(current, &after) != nil {
		return false
	}
	if !removeKnownVelaManagedMCPEntry(before) || !removeKnownVelaManagedMCPEntry(after) {
		return false
	}
	beforeData, beforeErr := json.Marshal(before)
	afterData, afterErr := json.Marshal(after)
	return beforeErr == nil && afterErr == nil && bytes.Equal(beforeData, afterData)
}

func removeKnownVelaManagedMCPEntry(config map[string]interface{}) bool {
	mcp, _ := config["mcp"].(map[string]interface{})
	entry, exists := mcp["vela"]
	if !exists {
		return true
	}
	managedEntry, ok := entry.(map[string]interface{})
	if !ok || !isProvenManagedMCPEntry(managedEntry) {
		return false
	}
	delete(mcp, "vela")
	return true
}

func (transaction *openCodeVelaTransaction) record(result openCodeVelaRollbackResult, outcome error) error {
	evidence := openCodeVelaRollbackEvidence{openCodeVelaRollbackResult: result}
	if outcome != nil {
		evidence.Error = outcome.Error()
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(transaction.backupDir, 0o750); err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Join(transaction.backupDir, "vela-rollback.json"), data, 0o600); err != nil {
		return err
	}
	return outcome
}
