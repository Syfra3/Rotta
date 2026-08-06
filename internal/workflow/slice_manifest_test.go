package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ093_Slice4ManifestSelfValidates(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadAndValidateSliceEvidenceManifest(root, ".rotta/strict/slice-4-remediation-cycle-2.json")
	if err != nil {
		t.Fatalf("slice-4 evidence manifest is not self-validating: %v", err)
	}
	if manifest.Slice != "slice-4" || manifest.PredecessorSlice != "slice-3" {
		t.Fatalf("slice attribution = %#v", manifest)
	}
	for _, entry := range manifest.Paths {
		if entry.Origin != "slice-4" || (entry.Disposition != "added" && entry.Disposition != "changed") {
			t.Fatalf("Slice-4 path was misattributed as inherited: %#v", entry)
		}
	}
}

func TestREQ096_Slice6ManifestSelfValidatesUntrackedFiles(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadAndValidateSliceEvidenceManifest(root, ".rotta/strict/slice-6-remediation-cycle-1.json")
	if err != nil {
		t.Fatalf("slice-6 evidence manifest is not self-validating: %v", err)
	}
	if manifest.Slice != "slice-6" || manifest.PredecessorSlice != "slice-5" {
		t.Fatalf("slice attribution = %#v", manifest)
	}
	for _, entry := range manifest.Paths {
		if entry.Origin != "slice-6" || (entry.Tracking == "untracked" && entry.Disposition != "added") {
			t.Fatalf("Slice-6 path attribution is invalid: %#v", entry)
		}
	}
}

func TestREQ093_SliceEvidenceManifestValidatesUntrackedContentWithoutGit(t *testing.T) {
	root := t.TempDir()
	path := "internal/untracked.go"
	contents := []byte("package fixture\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	manifest := SliceEvidenceManifest{Format: sliceEvidenceManifestFormat, Slice: "slice-4", PredecessorSlice: "slice-3", Paths: []SliceEvidencePath{{Path: path, SHA256: hex.EncodeToString(sum[:]), Origin: "slice-4", Disposition: "added", Tracking: "untracked"}}, Verification: []SliceEvidenceVerification{{Command: "go test ./internal/workflow", Result: "passed"}}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "slice.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndValidateSliceEvidenceManifest(root, "slice.json"); err != nil {
		t.Fatalf("untracked content manifest did not validate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndValidateSliceEvidenceManifest(root, "slice.json"); err == nil {
		t.Fatal("mutated listed file passed manifest validation")
	}
}

func TestREQ093_SliceEvidenceManifestRejectsMalformedOrMisattributedPaths(t *testing.T) {
	root := t.TempDir()
	path := "internal/fixture.go"
	contents := []byte("package fixture\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	base := SliceEvidenceManifest{Format: sliceEvidenceManifestFormat, Slice: "slice-4", PredecessorSlice: "slice-3", Paths: []SliceEvidencePath{{Path: path, SHA256: hex.EncodeToString(sum[:]), Origin: "slice-4", Disposition: "added", Tracking: "untracked"}}, Verification: []SliceEvidenceVerification{{Command: "go test ./internal/workflow", Result: "passed"}}}
	for name, mutate := range map[string]func(*SliceEvidenceManifest){
		"Slice-4 called inherited": func(manifest *SliceEvidenceManifest) { manifest.Paths[0].Disposition = "inherited" },
		"predecessor called changed": func(manifest *SliceEvidenceManifest) {
			manifest.Paths[0].Origin, manifest.Paths[0].Disposition, manifest.Paths[0].Tracking = "slice-3", "changed", "tracked"
		},
		"untracked called changed": func(manifest *SliceEvidenceManifest) { manifest.Paths[0].Disposition = "changed" },
		"unknown origin":           func(manifest *SliceEvidenceManifest) { manifest.Paths[0].Origin = "slice-9" },
		"bad hash":                 func(manifest *SliceEvidenceManifest) { manifest.Paths[0].SHA256 = strings.Repeat("0", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			manifest := base
			manifest.Paths = append([]SliceEvidencePath(nil), base.Paths...)
			mutate(&manifest)
			if err := ValidateSliceEvidenceManifest(root, manifest); err == nil {
				t.Fatal("malformed or misattributed manifest was accepted")
			}
		})
	}

	data, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data[:len(data)-1], []byte(`,"unexpected":"field"}`)...)
	if err := os.WriteFile(filepath.Join(root, "slice.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAndValidateSliceEvidenceManifest(root, "slice.json"); err == nil {
		t.Fatal("manifest with an unknown field was accepted")
	}
}

func TestREQ093_SliceEvidenceManifestAllowsTrackedInheritedSliceTwoAndThreePaths(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"internal/slice2.go", "internal/slice3.go"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte("package fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	contents := []byte("package fixture\n")
	sum := sha256.Sum256(contents)
	manifest := SliceEvidenceManifest{Format: sliceEvidenceManifestFormat, Slice: "slice-4", PredecessorSlice: "slice-3", Paths: []SliceEvidencePath{
		{Path: "internal/slice2.go", SHA256: hex.EncodeToString(sum[:]), Origin: "slice-2", Disposition: "inherited", Tracking: "tracked"},
		{Path: "internal/slice3.go", SHA256: hex.EncodeToString(sum[:]), Origin: "slice-3", Disposition: "inherited", Tracking: "tracked"},
	}, Verification: []SliceEvidenceVerification{{Command: "go test ./internal/workflow", Result: "passed"}}}
	if err := ValidateSliceEvidenceManifest(root, manifest); err != nil {
		t.Fatalf("valid inherited path attribution was rejected: %v", err)
	}
}
