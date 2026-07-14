package workflow

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readRepositoryFile(repoRoot, path string) ([]byte, error) {
	file, closeFile, err := openRepositoryFile(repoRoot, path)
	if err != nil {
		return nil, err
	}
	defer closeFile()
	return io.ReadAll(file)
}

func repositoryFileExists(repoRoot, path string) error {
	_, closeFile, err := openRepositoryFile(repoRoot, path)
	if err != nil {
		return err
	}
	defer closeFile()
	return nil
}

func openRepositoryFile(repoRoot, path string) (*os.File, func() error, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, nil, os.ErrNotExist
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	file, err := root.Open(clean)
	if err != nil {
		_ = root.Close()
		return nil, nil, err
	}
	return file, func() error { _ = file.Close(); return root.Close() }, nil
}
