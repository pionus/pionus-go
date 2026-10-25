package typecho

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// extractBundle unpacks a migration bundle and locates the Typecho JSON and
// uploads directory. A bundle may contain a top-level directory:
//
//	typecho.json
//	uploads/...
//
// or:
//
//	usr/uploads/...
func ExtractBundle(path string) (dir, exportPath, uploads string, err error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", "", "", fmt.Errorf("open bundle: %w", err)
	}
	defer reader.Close()

	dir, err = os.MkdirTemp("", "pionus-typecho-")
	if err != nil {
		return "", "", "", fmt.Errorf("create bundle temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	for _, file := range reader.File {
		name := filepath.ToSlash(file.Name)
		clean := filepath.Clean(name)
		if name == "" || strings.HasPrefix(name, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			cleanup()
			return "", "", "", fmt.Errorf("bundle contains unsafe path %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(dir, clean), 0755); err != nil {
				cleanup()
				return "", "", "", fmt.Errorf("create bundle directory: %w", err)
			}
			continue
		}
		if !file.Mode().IsRegular() {
			cleanup()
			return "", "", "", fmt.Errorf("bundle contains unsupported entry %q", file.Name)
		}
		dst := filepath.Join(dir, clean)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			cleanup()
			return "", "", "", fmt.Errorf("create bundle file directory: %w", err)
		}
		src, err := file.Open()
		if err != nil {
			cleanup()
			return "", "", "", fmt.Errorf("open bundle entry %q: %w", file.Name, err)
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			cleanup()
			return "", "", "", fmt.Errorf("create bundle file %q: %w", file.Name, err)
		}
		_, copyErr := io.Copy(out, src)
		closeErr := out.Close()
		src.Close()
		if copyErr != nil {
			cleanup()
			return "", "", "", fmt.Errorf("extract bundle entry %q: %w", file.Name, copyErr)
		}
		if closeErr != nil {
			cleanup()
			return "", "", "", fmt.Errorf("close bundle file %q: %w", file.Name, closeErr)
		}
	}

	exportPath, err = findExport(dir)
	if err != nil {
		cleanup()
		return "", "", "", err
	}
	uploads = findUploads(dir)
	return dir, exportPath, uploads, nil
}

func findExport(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(entry.Name()) {
		case "typecho.json", "typecho2.json":
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan bundle for export: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("bundle does not contain typecho.json or typecho2.json")
	}
	return found, nil
}

func findUploads(dir string) string {
	for _, rel := range []string{"usr/uploads", "uploads"} {
		path := filepath.Join(dir, rel)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	// Support a bundle wrapped in one top-level directory.
	var found string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if entry.Name() == "uploads" || filepath.ToSlash(path) == filepath.ToSlash(filepath.Join(dir, "usr", "uploads")) {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found
}
