package typecho

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.zip")
	writeZip(t, path, map[string]string{
		"export/typecho.json":  `{"site":{}}`,
		"export/uploads/a.jpg": "image",
		"export/readme.txt":    "ignore",
	})

	dir, exportPath, uploads, err := ExtractBundle(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if filepath.Base(exportPath) != "typecho.json" {
		t.Fatalf("export = %q", exportPath)
	}
	if uploads == "" {
		t.Fatal("uploads dir not found")
	}
	if _, err := os.Stat(filepath.Join(uploads, "a.jpg")); err != nil {
		t.Fatalf("upload missing: %v", err)
	}
}

func TestExtractBundleRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.zip")
	writeZip(t, path, map[string]string{
		"../evil.json": `{}`,
	})
	if _, _, _, err := ExtractBundle(path); err == nil {
		t.Fatal("expected unsafe path error")
	}
}
