package httpapi

import (
	"archive/zip"
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

func importAdventureZip(ctx context.Context, store *Store, uploadsDir string, filePath string, adventure Adventure) (ZipImportReport, error) {
	if err := validateZipArchive(filePath, 200, 120<<20); err != nil {
		return ZipImportReport{}, err
	}

	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return ZipImportReport{}, err
	}
	defer reader.Close()

	targetRoot := filepath.Join(uploadsDir, "imports", adventure.ID)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return ZipImportReport{}, err
	}

	report := ZipImportReport{
		Adventure: adventure,
		Documents: []Document{},
		Assets:    []Asset{},
	}

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		safeZipPath, err := secureZipRelativePath(file.Name)
		if err != nil {
			return report, err
		}
		assetType, entityName, locationName, tags := classifyImportedPath(safeZipPath)
		targetPath := filepath.Join(targetRoot, safeZipPath)
		if err := ensurePathWithinRoot(targetRoot, targetPath); err != nil {
			return report, err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return report, err
		}

		if err := extractZipFile(file, targetPath); err != nil {
			return report, err
		}

		extension := strings.ToLower(filepath.Ext(safeZipPath))
		mimeType := mime.TypeByExtension(extension)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		if extension == ".pdf" {
			document, err := store.CreateDocument(ctx, CreateDocumentRequest{
				AdventureID:    &adventure.ID,
				Type:           inferDocumentType(safeZipPath),
				Name:           filepath.Base(safeZipPath),
				SourceFilePath: &targetPath,
				Metadata: map[string]any{
					"source_type": "zip_import",
					"zip_path":    safeZipPath,
				},
			})
			if err != nil {
				return report, err
			}

			text, err := extractDocumentText(targetPath)
			if err == nil {
				chunks := chunkDocumentText(text, 1200)
				if len(chunks) > 0 {
					if err := store.ReplaceDocumentChunks(ctx, document.ID, chunks, map[string]any{
						"source_type": "zip_import",
						"zip_path":    safeZipPath,
					}); err == nil {
						document.ChunkCount = len(chunks)
					}
					if entries := extractRuleIndexEntries(document.Name, chunks); len(entries) > 0 {
						_ = store.ReplaceRuleIndexEntries(ctx, document.ID, entries)
					}
				}
			}

			report.Documents = append(report.Documents, document)
			report.Summary.ImportedDocuments++
			continue
		}

		asset, err := store.CreateAsset(ctx, Asset{
			AdventureID:  &adventure.ID,
			Type:         assetType,
			SourceType:   "manual_zip_import",
			Name:         filepath.Base(safeZipPath),
			FilePath:     targetPath,
			MimeType:     mimeType,
			EntityName:   entityName,
			LocationName: locationName,
			Tags:         tags,
			Metadata: map[string]any{
				"zip_path": safeZipPath,
			},
		})
		if err != nil {
			return report, err
		}

		report.Assets = append(report.Assets, asset)
		report.Summary.ImportedAssets++
		switch assetType {
		case "battlemap":
			report.Summary.ImportedBattlemaps++
		case "portrait":
			report.Summary.ImportedPortraits++
		case "token":
			report.Summary.ImportedTokens++
		case "handout", "printable":
			report.Summary.ImportedHandouts++
		}
	}

	return report, nil
}

func extractZipFile(file *zip.File, targetPath string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, reader)
	return err
}

func classifyImportedPath(path string) (assetType string, entityName *string, locationName *string, tags []string) {
	lower := strings.ToLower(path)
	tags = []string{}
	assetType = "image"

	switch {
	case strings.Contains(lower, "battlemap"):
		assetType = "battlemap"
		tags = append(tags, "battlemap")
	case strings.Contains(lower, "portrait"):
		assetType = "portrait"
		tags = append(tags, "portrait")
	case strings.Contains(lower, "token"):
		assetType = "token"
		tags = append(tags, "token")
	case strings.Contains(lower, "paperminis"):
		assetType = "printable"
		tags = append(tags, "printable")
	case strings.Contains(lower, "brief") || strings.Contains(lower, "letter"):
		assetType = "handout"
		tags = append(tags, "handout")
	case strings.Contains(lower, "map"):
		assetType = "map"
		tags = append(tags, "map")
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")

	if strings.Contains(lower, "ghost knight fellehar") || strings.Contains(lower, "ghost_knight_fellehar") {
		value := "Ghost Knight Fellehar"
		entityName = &value
		tags = append(tags, "fellehar")
	}
	if strings.Contains(lower, "brother benjamin") || strings.Contains(lower, "brother_benjamin") {
		value := "Brother Benjamin"
		entityName = &value
		tags = append(tags, "brother_benjamin")
	}
	if strings.Contains(lower, "nara the hag") || strings.Contains(lower, "nara_the_hag") {
		value := "Nara the Hag"
		entityName = &value
		tags = append(tags, "nara")
	}
	if strings.Contains(lower, "abbeycrypt") {
		value := "Abbey Crypt"
		locationName = &value
		tags = append(tags, "abbey_crypt")
	}
	if strings.Contains(lower, "abbeyfirstfloor") {
		value := "Abbey First Floor"
		locationName = &value
		tags = append(tags, "abbey_first_floor")
	}
	if strings.Contains(lower, "abbeyroof") {
		value := "Abbey Roof"
		locationName = &value
		tags = append(tags, "abbey_roof")
	}

	if entityName == nil && assetType == "portrait" {
		value := strings.TrimSpace(base)
		entityName = &value
	}

	return assetType, entityName, locationName, tags
}

func inferDocumentType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "papermini"):
		return "asset"
	case strings.Contains(lower, "handout"):
		return "asset"
	default:
		return "adventure"
	}
}
