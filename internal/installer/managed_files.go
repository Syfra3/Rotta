package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const managedArtifactsVersion = 1

type managedArtifactsManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

// installManagedFiles only updates files previously recorded by Rotta. This
// prevents an installer upgrade from silently replacing user-owned prompts.
func installManagedFiles(home string, files map[string][]byte) ([]string, error) {
	manifest, err := validateManagedFiles(home, files)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(files))
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create managed artifact directory: %w", err)
		}
		if err := validateManagedParents(home, path); err != nil {
			return nil, err
		}
		if err := writePrivateFile(path, data, 0o600); err != nil {
			return nil, fmt.Errorf("write managed artifact %s: %w", path, err)
		}
		manifest.Files[path] = contentDigest(data)
		paths = append(paths, path)
	}

	if err := writeManagedArtifactsManifest(home, manifest); err != nil {
		return nil, err
	}
	return paths, nil
}

func validateManagedFiles(home string, files map[string][]byte) (managedArtifactsManifest, error) {
	manifestPath := filepath.Join(home, ".config", "rotta", "managed-artifacts.json")
	manifest, err := readManagedArtifactsManifest(manifestPath)
	if err != nil {
		return managedArtifactsManifest{}, err
	}
	for path := range files {
		if err := validateManagedParents(home, path); err != nil {
			return managedArtifactsManifest{}, err
		}
		if err := validateManagedTarget(path, manifest.Files); err != nil {
			return managedArtifactsManifest{}, err
		}
	}
	return manifest, nil
}

func writeManagedArtifactsManifest(home string, manifest managedArtifactsManifest) error {
	manifestPath := filepath.Join(home, ".config", "rotta", "managed-artifacts.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize managed artifact manifest: %w", err)
	}
	if err := validateManagedParents(home, manifestPath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o750); err != nil {
		return fmt.Errorf("create managed artifact manifest directory: %w", err)
	}
	if err := validateManagedParents(home, manifestPath); err != nil {
		return err
	}
	if info, err := os.Lstat(manifestPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed artifact manifest must not be a symlink: %s", manifestPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect managed artifact manifest: %w", err)
	}
	if err := writePrivateFile(manifestPath, data, 0o600); err != nil {
		return fmt.Errorf("write managed artifact manifest: %w", err)
	}
	return nil
}

func validateManagedParents(home, path string) error {
	relative, err := filepath.Rel(home, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("managed artifact path is outside home: %s", path)
	}
	parent := filepath.Dir(relative)
	current := home
	for _, part := range strings.Split(parent, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect managed artifact directory %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed artifact directory must not be a symlink: %s", current)
		}
	}
	return nil
}

func readManagedArtifactsManifest(path string) (managedArtifactsManifest, error) {
	manifest := managedArtifactsManifest{Version: managedArtifactsVersion, Files: map[string]string{}}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return manifest, nil
	}
	if err != nil {
		return manifest, fmt.Errorf("inspect managed artifact manifest: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return manifest, fmt.Errorf("managed artifact manifest must not be a symlink: %s", path)
	}
	data, err := readPrivateFile(path)
	if err != nil {
		return manifest, fmt.Errorf("read managed artifact manifest: %w", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != managedArtifactsVersion || manifest.Files == nil {
		return managedArtifactsManifest{}, fmt.Errorf("managed artifact manifest is malformed: %s", path)
	}
	for managedPath, digest := range manifest.Files {
		if managedPath == "" || !validDigest(digest) {
			return managedArtifactsManifest{}, fmt.Errorf("managed artifact manifest is malformed: %s", path)
		}
	}
	return manifest, nil
}

func validateManagedTarget(path string, owned map[string]string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect managed artifact %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed artifact must not be a symlink: %s", path)
	}
	want, exists := owned[path]
	if !exists {
		return fmt.Errorf("refusing to overwrite unmanaged artifact: %s", path)
	}
	data, err := readPrivateFile(path)
	if err != nil {
		return fmt.Errorf("read managed artifact %s: %w", path, err)
	}
	if contentDigest(data) != want {
		return fmt.Errorf("refusing to overwrite modified managed artifact: %s", path)
	}
	return nil
}

func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
