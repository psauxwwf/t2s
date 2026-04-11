package fs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func WriteFile(
	path string,
	data []byte,
	perm ...int,
) error {
	if len(perm) == 0 {
		perm = append(perm, 0644)
	}
	if err := os.WriteFile(path, data, fs.FileMode(perm[0])); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	return nil
}

func ReadFile(path string) ([]byte, error) {
	f, _, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err = buf.ReadFrom(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func OpenFile(filename string) (*os.File, fs.FileInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("error file %s not open: %w", filename, err)
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("error to get file size: %w", err)
	}
	return f, stat, nil
}

func CheckSymlink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	return target, nil
}

func CopyFile(src, dst string, perm ...int) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}

	_src, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error file %s not open: %w", src, err)
	}
	defer _src.Close()

	mode := fs.FileMode(0)
	if len(perm) > 0 {
		mode = fs.FileMode(perm[0])
	} else {
		stat, err := _src.Stat()
		if err != nil {
			return fmt.Errorf("error to get file size: %w", err)
		}
		mode = stat.Mode().Perm()
	}

	_dst, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("error file %s not open: %w", dst, err)
	}

	if _, err := io.Copy(_dst, _src); err != nil {
		_dst.Close()
		return fmt.Errorf("copy file %s -> %s: %w", src, dst, err)
	}

	if err := _dst.Sync(); err != nil {
		_dst.Close()
		return fmt.Errorf("sync file %s: %w", dst, err)
	}

	if err := _dst.Close(); err != nil {
		return fmt.Errorf("close file %s: %w", dst, err)
	}

	return nil
}
