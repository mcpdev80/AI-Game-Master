package httpapi

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	errZipPathTraversal   = errors.New("zip entry path traversal is not allowed")
	errZipTooManyEntries  = errors.New("zip archive contains too many entries")
	errZipTooLarge        = errors.New("zip archive exceeds extraction size limit")
	errUploadTooLarge     = errors.New("uploaded file exceeds size limit")
	errUploadTypeRejected = errors.New("uploaded file type is not allowed")
)

var allowedDocumentExtensions = map[string]struct{}{
	".pdf":  {},
	".md":   {},
	".txt":  {},
	".yaml": {},
	".yml":  {},
	".json": {},
}

var allowedCharacterSheetExtensions = map[string]struct{}{
	".pdf": {},
	".md":  {},
	".txt": {},
}

var allowedAssetExtensions = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".webp": {},
	".gif":  {},
	".svg":  {},
	".mp3":  {},
	".wav":  {},
	".ogg":  {},
	".mp4":  {},
	".webm": {},
}

var allowedZipAssetExtensions = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".webp": {},
	".gif":  {},
	".svg":  {},
}

var allowedAudioExtensions = map[string]struct{}{
	".wav":  {},
	".mp3":  {},
	".m4a":  {},
	".mp4":  {},
	".mpeg": {},
	".webm": {},
	".ogg":  {},
}

func ensureUploadSize(fileHeader *multipart.FileHeader, maxBytes int64) error {
	if fileHeader == nil {
		return errors.New("uploaded file is missing")
	}
	if maxBytes > 0 && fileHeader.Size > maxBytes {
		return fmt.Errorf("%w: %d > %d", errUploadTooLarge, fileHeader.Size, maxBytes)
	}
	return nil
}

func ensureAllowedExtension(filename string, allowed map[string]struct{}) error {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if _, ok := allowed[extension]; !ok {
		return fmt.Errorf("%w: %s", errUploadTypeRejected, extension)
	}
	return nil
}

func saveUploadedFileChecked(fileHeader *multipart.FileHeader, targetPath string, maxBytes int64) error {
	if err := ensureUploadSize(fileHeader, maxBytes); err != nil {
		return err
	}
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	limitReader := io.LimitReader(src, maxBytes+1)
	written, err := io.Copy(target, limitReader)
	if err != nil {
		return err
	}
	if maxBytes > 0 && written > maxBytes {
		return errUploadTooLarge
	}
	return nil
}

func validateZipArchive(filePath string, maxEntries int, maxExtractBytes int64) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if maxEntries > 0 && len(reader.File) > maxEntries {
		return errZipTooManyEntries
	}

	var totalUncompressed int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if _, err := secureZipRelativePath(file.Name); err != nil {
			return err
		}
		extension := strings.ToLower(filepath.Ext(file.Name))
		if extension == ".pdf" {
			totalUncompressed += int64(file.UncompressedSize64)
			continue
		}
		if _, ok := allowedZipAssetExtensions[extension]; !ok {
			return fmt.Errorf("%w: %s", errUploadTypeRejected, extension)
		}
		totalUncompressed += int64(file.UncompressedSize64)
		if maxExtractBytes > 0 && totalUncompressed > maxExtractBytes {
			return errZipTooLarge
		}
	}

	return nil
}

func secureZipRelativePath(name string) (string, error) {
	raw := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if raw == "" {
		return "", errZipPathTraversal
	}
	cleaned := filepath.Clean(raw)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", errZipPathTraversal
	}
	if filepath.IsAbs(cleaned) {
		return "", errZipPathTraversal
	}
	parts := strings.Split(filepath.ToSlash(cleaned), "/")
	sanitized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			return "", errZipPathTraversal
		}
		safe := sanitizeFilename(part)
		if safe == "" || safe == "." || safe == ".." {
			return "", errZipPathTraversal
		}
		sanitized = append(sanitized, safe)
	}
	if len(sanitized) == 0 {
		return "", errZipPathTraversal
	}
	return filepath.Join(sanitized...), nil
}

func ensurePathWithinRoot(root string, candidate string) error {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return err
	}
	normalized := filepath.ToSlash(rel)
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return errZipPathTraversal
	}
	return nil
}

func allowedAudioContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		return false
	}
	return strings.HasPrefix(contentType, "audio/") || contentType == "video/webm" || contentType == "application/octet-stream"
}

func classifyUploadValidation(documentType string) map[string]struct{} {
	switch documentType {
	case "character_sheet":
		return allowedCharacterSheetExtensions
	case "asset":
		return allowedAssetExtensions
	default:
		return allowedDocumentExtensions
	}
}

func uploadErrorStatus(err error) int {
	switch {
	case errors.Is(err, errUploadTooLarge), errors.Is(err, errUploadTypeRejected), errors.Is(err, errZipTooManyEntries), errors.Is(err, errZipTooLarge), errors.Is(err, errZipPathTraversal):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
