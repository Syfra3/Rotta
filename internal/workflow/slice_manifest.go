package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const sliceEvidenceManifestFormat = "rotta.slice-evidence/v2"

// SliceEvidenceManifest is an explicit content inventory, not a Git diff
// claim. It therefore remains useful when a permitted slice contains untracked
// files or accumulated worktree changes.
type SliceEvidenceManifest struct {
	Format           string                      `json:"format"`
	Slice            string                      `json:"slice"`
	PredecessorSlice string                      `json:"predecessor_slice"`
	Paths            []SliceEvidencePath         `json:"paths"`
	Verification     []SliceEvidenceVerification `json:"verification"`
}

type SliceEvidencePath struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Origin      string `json:"origin"`
	Disposition string `json:"disposition"`
	Tracking    string `json:"tracking"`
}

type SliceEvidenceVerification struct {
	Command string `json:"command"`
	Result  string `json:"result"`
}

// LoadAndValidateSliceEvidenceManifest validates the bounded record and each
// listed file's current content hash. It intentionally does not call Git.
func LoadAndValidateSliceEvidenceManifest(root, manifestPath string) (SliceEvidenceManifest, error) {
	if !canonicalWorkflowPath(manifestPath) {
		return SliceEvidenceManifest{}, fmt.Errorf("slice evidence manifest path is not repository-confined")
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifestPath)))
	if err != nil {
		return SliceEvidenceManifest{}, err
	}
	var manifest SliceEvidenceManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return SliceEvidenceManifest{}, fmt.Errorf("decode slice evidence manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return SliceEvidenceManifest{}, fmt.Errorf("decode slice evidence manifest: trailing data")
	}
	if err := ValidateSliceEvidenceManifest(root, manifest); err != nil {
		return SliceEvidenceManifest{}, err
	}
	return manifest, nil
}

func ValidateSliceEvidenceManifest(root string, manifest SliceEvidenceManifest) error {
	if manifest.Format != sliceEvidenceManifestFormat || !validSliceIdentity(manifest.Slice, manifest.PredecessorSlice) {
		return fmt.Errorf("slice evidence manifest has invalid identity")
	}
	if len(manifest.Paths) == 0 || len(manifest.Paths) > 64 || len(manifest.Verification) == 0 || len(manifest.Verification) > 16 {
		return fmt.Errorf("slice evidence manifest exceeds bounded path or verification count")
	}
	paths := make([]string, 0, len(manifest.Paths))
	for _, entry := range manifest.Paths {
		if !canonicalWorkflowPath(entry.Path) || len(entry.SHA256) != 64 || !validSlicePathAttribution(manifest.Slice, entry) {
			return fmt.Errorf("slice evidence manifest has unsafe path attribution or hash")
		}
		if _, err := hex.DecodeString(entry.SHA256); err != nil || strings.ToLower(entry.SHA256) != entry.SHA256 {
			return fmt.Errorf("slice evidence manifest has non-canonical content hash")
		}
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.Path)))
		if err != nil {
			return fmt.Errorf("read listed slice path %q: %w", entry.Path, err)
		}
		sum := sha256.Sum256(contents)
		if entry.SHA256 != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("listed slice path %q does not match its manifest hash", entry.Path)
		}
		paths = append(paths, entry.Path)
	}
	if !sort.StringsAreSorted(paths) || hasDuplicateManifestPath(paths) {
		return fmt.Errorf("slice evidence manifest paths must be sorted and unique")
	}
	for _, verification := range manifest.Verification {
		if !compactManifestText(verification.Command) || !compactManifestText(verification.Result) {
			return fmt.Errorf("slice evidence manifest has non-compact verification data")
		}
	}
	return nil
}

func validSliceIdentity(slice, predecessor string) bool {
	parse := func(value string) (int, bool) {
		if !strings.HasPrefix(value, "slice-") {
			return 0, false
		}
		number, err := strconv.Atoi(strings.TrimPrefix(value, "slice-"))
		return number, err == nil && number > 1 && value == "slice-"+strconv.Itoa(number)
	}
	current, ok := parse(slice)
	previous, previousOK := parse(predecessor)
	return ok && previousOK && previous == current-1
}

func validSlicePathAttribution(slice string, entry SliceEvidencePath) bool {
	if entry.Tracking != "tracked" && entry.Tracking != "untracked" {
		return false
	}
	if entry.Origin == slice {
		if entry.Disposition != "added" && entry.Disposition != "changed" {
			return false
		}
		// An untracked path cannot be an inherited or modified predecessor
		// path; its manifest claim is explicitly a current-slice addition.
		return entry.Tracking != "untracked" || entry.Disposition == "added"
	}
	current, currentOK := sliceNumber(slice)
	origin, originOK := sliceNumber(entry.Origin)
	return currentOK && originOK && origin < current && entry.Disposition == "inherited" && entry.Tracking == "tracked"
}

func sliceNumber(value string) (int, bool) {
	if !strings.HasPrefix(value, "slice-") {
		return 0, false
	}
	number, err := strconv.Atoi(strings.TrimPrefix(value, "slice-"))
	return number, err == nil && number > 1 && value == "slice-"+strconv.Itoa(number)
}

func compactManifestText(value string) bool {
	return value != "" && len(value) <= 512 && validateCompactText("manifest field", value) == nil
}

func hasDuplicateManifestPath(paths []string) bool {
	for index := 1; index < len(paths); index++ {
		if paths[index-1] == paths[index] {
			return true
		}
	}
	return false
}
