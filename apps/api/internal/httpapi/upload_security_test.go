package httpapi

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestSecureZipRelativePathRejectsTraversal(t *testing.T) {
	if _, err := secureZipRelativePath("../secrets.txt"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestSecureZipRelativePathSanitizesNormalPath(t *testing.T) {
	got, err := secureZipRelativePath("maps/crypt map.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join("maps", "crypt_map.png") {
		t.Fatalf("unexpected sanitized path: %q", got)
	}
}

func TestEnsureAllowedExtensionRejectsUnknownType(t *testing.T) {
	if err := ensureAllowedExtension("malware.exe", allowedAssetExtensions); err == nil {
		t.Fatal("expected unknown extension to be rejected")
	}
}

func TestAllowedAudioContentTypeRejectsTextPlain(t *testing.T) {
	if allowedAudioContentType("text/plain") {
		t.Fatal("expected text/plain to be rejected")
	}
}

func TestValidateZipArchiveRejectsTraversalEntry(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "traversal.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"../secrets.txt": "oops",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	if err := validateZipArchive(zipPath, 10, 1<<20); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}

func TestValidateZipArchiveRejectsUnsupportedExtension(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "bad-ext.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"maps/script.js": "alert(1)",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	if err := validateZipArchive(zipPath, 10, 1<<20); err == nil {
		t.Fatal("expected unsupported extension to be rejected")
	}
}

func writeTestZip(target string, files map[string]string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			_ = writer.Close()
			return err
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			_ = writer.Close()
			return err
		}
	}
	return writer.Close()
}
