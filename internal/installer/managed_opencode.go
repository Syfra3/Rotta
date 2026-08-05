package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const managedOpenCodeManifest = ".rotta/rotta-manifest.json"

var managedOpenCodeAssets = map[string][]byte{
	"agents/rotta-orchestrator.md": []byte(`---
name: rotta-orchestrator
description: Coordinate the Rotta contract-driven workflow.
mode: primary
---

# Rotta Orchestrator

Use approved contracts and durable records beneath .rotta/workflow as the workflow authority. Delegate specification, implementation, and review work to the corresponding Rotta agents. Only the orchestrator authorizes lifecycle transitions.
`),
	"agents/rotta-spec.md": []byte(`---
name: rotta-spec
description: Define hard specifications and Gherkin contracts for Rotta.
mode: subagent
---

# Rotta Spec

Write only assigned specification and Gherkin artifacts. Use .rotta/workflow only as a durable workflow reference; do not create approvals, transitions, or lifecycle records.
`),
	"agents/rotta-impl.md": []byte(`---
name: rotta-impl
description: Implement one approved Rotta scenario with strict TDD.
mode: subagent
---

# Rotta Implementation

Implement one approved scenario at a time using Red, Green, and Refactor. Read approved scope and task state from .rotta/workflow; return evidence to the orchestrator without authorizing lifecycle transitions.
`),
	"agents/rotta-review.md": []byte(`---
name: rotta-review
description: Evaluate Rotta implementation evidence against approved quality gates.
mode: subagent
---

# Rotta Review

Review bounded evidence for the approved scope recorded beneath .rotta/workflow. Return a pass, fail, or escalation recommendation to the orchestrator; never advance lifecycle state directly.
`),
}

// ManagedOpenCodeResult describes only assets owned by the minimal installer.
type ManagedOpenCodeResult struct {
	Files []string
}

// ManagedOpenCodeStatus is a bounded, read-only installation observation.
type ManagedOpenCodeStatus struct {
	State string
	Files []string
}

type managedOpenCodeManifestFile struct {
	Managed map[string]string `json:"managed"`
}

// InstallOpenCode installs only Rotta-owned assets below the user's OpenCode root.
func InstallOpenCode(home string) (*ManagedOpenCodeResult, error) {
	root := filepath.Join(home, ".config", "opencode")
	if err := ensureManagedOpenCodeRoot(root); err != nil {
		return nil, err
	}
	manifest, exists, err := readManagedOpenCodeManifest(root)
	if err != nil {
		return nil, err
	}
	if err := preflightManagedOpenCodeAssets(root, manifest, exists); err != nil {
		return nil, err
	}

	files := make([]string, 0, len(managedOpenCodeAssets)+1)
	for _, path := range managedOpenCodeAssetPaths() {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o700); err != nil {
			return nil, fmt.Errorf("create managed OpenCode directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(root, path), managedOpenCodeAssets[path], 0o600); err != nil {
			return nil, fmt.Errorf("write managed OpenCode asset %s: %w", path, err)
		}
		files = append(files, filepath.Join(root, path))
	}
	manifest.Managed = managedOpenCodeAssetDigests()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize managed OpenCode manifest: %w", err)
	}
	manifestPath := filepath.Join(root, managedOpenCodeManifest)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		return nil, fmt.Errorf("create managed manifest directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write managed OpenCode manifest: %w", err)
	}
	files = append(files, manifestPath)
	return &ManagedOpenCodeResult{Files: files}, nil
}

// OpenCodeStatus observes managed assets without creating or changing files.
func OpenCodeStatus(home string) ManagedOpenCodeStatus {
	root := filepath.Join(home, ".config", "opencode")
	manifest, exists, err := readManagedOpenCodeManifest(root)
	if os.IsNotExist(err) || !exists {
		return ManagedOpenCodeStatus{State: "not installed"}
	}
	if err != nil {
		return ManagedOpenCodeStatus{State: "uncertain"}
	}
	files := managedOpenCodeAssetPaths()
	for _, path := range files {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || manifest.Managed[path] != digestManagedOpenCodeAsset(data) {
			return ManagedOpenCodeStatus{State: "partial", Files: files}
		}
	}
	return ManagedOpenCodeStatus{State: "installed", Files: files}
}

func ensureManagedOpenCodeRoot(root string) error {
	info, err := os.Lstat(root)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("OpenCode configuration root must be a directory, not a symlink")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect OpenCode configuration root: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create OpenCode configuration root: %w", err)
	}
	return nil
}

func readManagedOpenCodeManifest(root string) (managedOpenCodeManifestFile, bool, error) {
	data, err := os.ReadFile(filepath.Join(root, managedOpenCodeManifest))
	if err != nil {
		if os.IsNotExist(err) {
			return managedOpenCodeManifestFile{}, false, nil
		}
		return managedOpenCodeManifestFile{}, false, err
	}
	var manifest managedOpenCodeManifestFile
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Managed == nil {
		return managedOpenCodeManifestFile{}, false, fmt.Errorf("managed OpenCode manifest is malformed; move or remove it before retrying")
	}
	return manifest, true, nil
}

func preflightManagedOpenCodeAssets(root string, manifest managedOpenCodeManifestFile, manifestExists bool) error {
	for _, path := range managedOpenCodeAssetPaths() {
		info, err := os.Lstat(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect managed OpenCode asset %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed OpenCode asset %s is a symlink; move or remove it before retrying", path)
		}
		if !manifestExists || manifest.Managed[path] == "" {
			return fmt.Errorf("unmanaged OpenCode configuration conflict at %s; move or remove it before retrying", path)
		}
	}
	return nil
}

func managedOpenCodeAssetPaths() []string {
	paths := make([]string, 0, len(managedOpenCodeAssets))
	for path := range managedOpenCodeAssets {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func managedOpenCodeAssetDigests() map[string]string {
	digests := make(map[string]string, len(managedOpenCodeAssets))
	for path, data := range managedOpenCodeAssets {
		digests[path] = digestManagedOpenCodeAsset(data)
	}
	return digests
}

func digestManagedOpenCodeAsset(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
