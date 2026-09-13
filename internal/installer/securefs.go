package installer

import (
	"io"
	"os"
	"path/filepath"
)

func readPrivateFile(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func writePrivateFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := os.CreateTemp(dir, ".rotta-tmp-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(perm); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func fileExistsWithinParent(path string) (bool, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	defer root.Close()
	_, err = root.Stat(filepath.Base(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
