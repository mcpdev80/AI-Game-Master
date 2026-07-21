package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) listDocuments(c *gin.Context) {
	items, err := h.store.ListDocuments(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list documents", err)
		return
	}

	hidden, err := h.store.ListHiddenSystemDocumentIDs(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list hidden system documents", err)
		return
	}

	items = mergeEmbeddedGuideDocuments(items, hidden)
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) listAssets(c *gin.Context) {
	items, err := h.store.ListAssets(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list assets", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) serveDocumentFile(c *gin.Context) {
	documents, err := h.store.ListDocuments(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list documents", err)
		return
	}
	hidden, err := h.store.ListHiddenSystemDocumentIDs(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list hidden system documents", err)
		return
	}
	documents = mergeEmbeddedGuideDocuments(documents, hidden)
	for _, document := range documents {
		if document.ID != c.Param("id") {
			continue
		}
		if embeddedContent := safeOptionalString(document.Metadata["embedded_content"]); embeddedContent != "" {
			contentType := safeOptionalString(document.Metadata["embedded_content_type"])
			if contentType == "" {
				contentType = "text/plain; charset=utf-8"
			}
			extension := ".txt"
			if strings.Contains(contentType, "markdown") {
				extension = ".md"
			} else if strings.Contains(contentType, "yaml") {
				extension = ".yaml"
			}
			c.Header("Content-Type", contentType)
			c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s%s"`, sanitizeFilename(document.Name), extension))
			c.String(http.StatusOK, embeddedContent)
			return
		}
		if guideContent := safeOptionalString(document.Metadata["guide_content"]); guideContent != "" {
			c.Header("Content-Type", "application/yaml; charset=utf-8")
			c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.yaml"`, sanitizeFilename(document.Name)))
			c.String(http.StatusOK, guideContent)
			return
		}
		if document.SourceFilePath == nil {
			break
		}
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(*document.SourceFilePath)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		c.Header("Content-Type", contentType)
		c.File(*document.SourceFilePath)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "document file not found"})
}

func (h *Handler) deleteDocument(c *gin.Context) {
	hidden, hiddenErr := h.store.ListHiddenSystemDocumentIDs(c.Request.Context())
	if hiddenErr != nil {
		errorResponse(c, http.StatusInternalServerError, "list hidden system documents", hiddenErr)
		return
	}
	for _, embedded := range embeddedBuilderGuideDocuments() {
		if embedded.ID != c.Param("id") {
			continue
		}
		if _, alreadyHidden := hidden[embedded.ID]; alreadyHidden {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		if err := h.store.HideSystemDocument(c.Request.Context(), embedded.ID); err != nil {
			errorResponse(c, http.StatusInternalServerError, "hide system document", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": true, "id": embedded.ID})
		return
	}

	document, err := h.store.GetDocument(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load document", err)
		return
	}
	if err := h.store.DeleteDocument(c.Request.Context(), document.ID); err != nil {
		errorResponse(c, http.StatusInternalServerError, "delete document", err)
		return
	}
	removeLocalFile(document.SourceFilePath)
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": document.ID})
}

func (h *Handler) serveAssetFile(c *gin.Context) {
	assets, err := h.store.ListAssets(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list assets", err)
		return
	}
	for _, asset := range assets {
		if asset.ID != c.Param("id") {
			continue
		}
		if asset.MimeType != "" {
			c.Header("Content-Type", asset.MimeType)
		}
		c.File(asset.FilePath)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "asset file not found"})
}

func (h *Handler) deleteAsset(c *gin.Context) {
	asset, err := h.store.GetAsset(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
			return
		}
		errorResponse(c, http.StatusInternalServerError, "load asset", err)
		return
	}
	if err := h.store.DeleteAsset(c.Request.Context(), asset.ID); err != nil {
		errorResponse(c, http.StatusInternalServerError, "delete asset", err)
		return
	}
	removeLocalFile(&asset.FilePath)
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": asset.ID})
}

func (h *Handler) createDocument(c *gin.Context) {
	var req CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid document payload", err)
		return
	}

	item, err := h.store.CreateDocument(c.Request.Context(), req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create document", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) reindexDocumentMonsters(c *gin.Context) {
	documentID := c.Param("id")

	documents, err := h.store.ListDocuments(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list documents", err)
		return
	}

	var document *Document
	for _, item := range documents {
		if item.ID == documentID {
			copy := item
			document = &copy
			break
		}
	}
	if document == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	chunks, err := h.store.ListDocumentChunks(c.Request.Context(), documentID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "list document chunks", err)
		return
	}

	chunkTexts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunkTexts = append(chunkTexts, chunk.ChunkText)
	}

	refs := extractMonsterReferences(document.Name, chunkTexts)
	if err := h.store.ReplaceMonsterReferences(c.Request.Context(), documentID, refs); err != nil {
		errorResponse(c, http.StatusInternalServerError, "replace monster references", err)
		return
	}
	if entries := extractRuleIndexEntries(document.Name, chunkTexts); len(entries) > 0 {
		if err := h.store.ReplaceRuleIndexEntries(c.Request.Context(), documentID, entries); err != nil {
			errorResponse(c, http.StatusInternalServerError, "replace rule index entries", err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id":      documentID,
		"document_name":    document.Name,
		"indexed_monsters": len(refs),
	})
}

func (h *Handler) createAdventurePackage(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	pdfHeader, err := c.FormFile("pdf")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "missing adventure pdf", err)
		return
	}
	if err := ensureAllowedExtension(pdfHeader.Filename, allowedDocumentExtensions); err != nil {
		errorResponse(c, uploadErrorStatus(err), "invalid adventure pdf type", err)
		return
	}
	if err := ensureUploadSize(pdfHeader, h.cfg.MaxUploadBytes); err != nil {
		errorResponse(c, uploadErrorStatus(err), "adventure pdf exceeds allowed size", err)
		return
	}

	if err := os.MkdirAll(h.uploadsDir, 0o755); err != nil {
		errorResponse(c, http.StatusInternalServerError, "prepare uploads directory", err)
		return
	}

	adventureMetadata := buildLibraryMetadataFromForm(c, true)
	adventure, err := h.store.CreateAdventure(c.Request.Context(), CreateAdventureRequest{
		CampaignID:  stringPtrOrNil(c.PostForm("campaign_id")),
		Name:        name,
		Description: c.PostForm("description"),
		Language:    firstNonEmpty(c.PostForm("language"), "de"),
		Metadata:    adventureMetadata,
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create adventure", err)
		return
	}

	pdfDocument, err := h.persistUploadedDocument(
		c,
		pdfHeader,
		CreateDocumentRequest{
			AdventureID: &adventure.ID,
			Type:        "adventure",
			Name:        firstNonEmpty(strings.TrimSpace(c.PostForm("pdf_name")), pdfHeader.Filename),
			Metadata: map[string]any{
				"source_type":         "adventure_pdf",
				"language":            firstNonEmpty(c.PostForm("language"), "de"),
				"ruleset_work":        adventureMetadata["ruleset_work"],
				"ruleset_version":     adventureMetadata["ruleset_version"],
				"ruleset_keys":        adventureMetadata["ruleset_keys"],
				"compatible_rulesets": adventureMetadata["compatible_rulesets"],
			},
		},
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "store adventure pdf", err)
		return
	}

	report := ZipImportReport{
		Adventure: adventure,
		Documents: []Document{pdfDocument},
		Assets:    []Asset{},
		Summary: ZipImportSummary{
			ImportedDocuments: 1,
		},
	}

	zipHeader, err := c.FormFile("resources_zip")
	if err == nil {
		if err := ensureAllowedExtension(zipHeader.Filename, map[string]struct{}{".zip": {}}); err != nil {
			errorResponse(c, uploadErrorStatus(err), "invalid resources archive type", err)
			return
		}
		if err := ensureUploadSize(zipHeader, h.cfg.MaxZipUploadBytes); err != nil {
			errorResponse(c, uploadErrorStatus(err), "resources archive exceeds allowed size", err)
			return
		}
		importDir := filepath.Join(h.uploadsDir, "zip-uploads")
		if err := os.MkdirAll(importDir, 0o755); err != nil {
			errorResponse(c, http.StatusInternalServerError, "prepare zip uploads directory", err)
			return
		}

		targetPath := filepath.Join(importDir, fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), sanitizeFilename(zipHeader.Filename)))
		if err := saveUploadedFileChecked(zipHeader, targetPath, h.cfg.MaxZipUploadBytes); err != nil {
			errorResponse(c, http.StatusInternalServerError, "store zip upload", err)
			return
		}
		if err := validateZipArchive(targetPath, h.cfg.MaxZipEntries, h.cfg.MaxZipExtractBytes); err != nil {
			errorResponse(c, uploadErrorStatus(err), "validate adventure resources archive", err)
			return
		}

		zipReport, err := importAdventureZip(c.Request.Context(), h.store, h.uploadsDir, targetPath, adventure, h.cfg.MaxZipEntries, h.cfg.MaxZipExtractBytes)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "import adventure resources", err)
			return
		}

		report.Documents = append(report.Documents, zipReport.Documents...)
		report.Assets = append(report.Assets, zipReport.Assets...)
		report.Summary.ImportedDocuments += zipReport.Summary.ImportedDocuments
		report.Summary.ImportedAssets += zipReport.Summary.ImportedAssets
		report.Summary.ImportedBattlemaps += zipReport.Summary.ImportedBattlemaps
		report.Summary.ImportedPortraits += zipReport.Summary.ImportedPortraits
		report.Summary.ImportedTokens += zipReport.Summary.ImportedTokens
		report.Summary.ImportedHandouts += zipReport.Summary.ImportedHandouts
	}

	c.JSON(http.StatusCreated, report)
}

func (h *Handler) uploadDocument(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "missing uploaded file", err)
		return
	}

	documentType := c.PostForm("type")
	if documentType == "" {
		documentType = "adventure"
	}
	if !validDocumentType(documentType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document type"})
		return
	}
	if err := ensureAllowedExtension(fileHeader.Filename, classifyUploadValidation(documentType)); err != nil {
		errorResponse(c, uploadErrorStatus(err), "invalid uploaded document type", err)
		return
	}
	if err := ensureUploadSize(fileHeader, h.cfg.MaxUploadBytes); err != nil {
		errorResponse(c, uploadErrorStatus(err), "uploaded document exceeds allowed size", err)
		return
	}

	name := c.PostForm("name")
	if name == "" {
		name = fileHeader.Filename
	}

	language := c.PostForm("language")
	campaignID := c.PostForm("campaign_id")
	adventureID := c.PostForm("adventure_id")

	metadata := map[string]any{
		"source_type": documentType,
	}
	if language != "" {
		metadata["language"] = language
	}
	if campaignID != "" {
		metadata["campaign_id"] = campaignID
	}
	if rawMetadata := c.PostForm("metadata_json"); rawMetadata != "" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(rawMetadata), &extra); err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid metadata_json", err)
			return
		}
		for key, value := range extra {
			metadata[key] = value
		}
	}
	for key, value := range buildLibraryMetadataFromForm(c, true) {
		metadata[key] = value
	}
	if documentType == "rules" {
		lowerFileName := strings.ToLower(strings.TrimSpace(fileHeader.Filename))
		lowerName := strings.ToLower(strings.TrimSpace(name))
		if lowerFileName == "short_rules.md" || lowerName == "short_rules" {
			metadata["kind"] = "short_rules_guide"
			metadata["source_type"] = "short_rules"
		}
	}

	if err := os.MkdirAll(h.uploadsDir, 0o755); err != nil {
		errorResponse(c, http.StatusInternalServerError, "prepare uploads directory", err)
		return
	}

	safeName := sanitizeFilename(fileHeader.Filename)
	targetName := fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), safeName)
	targetPath := filepath.Join(h.uploadsDir, targetName)

	if err := saveUploadedFileChecked(fileHeader, targetPath, h.cfg.MaxUploadBytes); err != nil {
		errorResponse(c, http.StatusInternalServerError, "store uploaded file", err)
		return
	}

	item, err := h.persistStoredDocument(c.Request.Context(), targetPath, CreateDocumentRequest{
		AdventureID: stringPtrOrNil(adventureID),
		Type:        documentType,
		Name:        name,
		Metadata:    metadata,
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create document record", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) uploadAsset(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "missing uploaded asset file", err)
		return
	}
	if err := ensureAllowedExtension(fileHeader.Filename, allowedAssetExtensions); err != nil {
		errorResponse(c, uploadErrorStatus(err), "invalid uploaded asset type", err)
		return
	}
	if err := ensureUploadSize(fileHeader, h.cfg.MaxUploadBytes); err != nil {
		errorResponse(c, uploadErrorStatus(err), "uploaded asset exceeds allowed size", err)
		return
	}

	if err := os.MkdirAll(h.uploadsDir, 0o755); err != nil {
		errorResponse(c, http.StatusInternalServerError, "prepare uploads directory", err)
		return
	}

	safeName := sanitizeFilename(fileHeader.Filename)
	targetName := fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), safeName)
	targetPath := filepath.Join(h.uploadsDir, targetName)
	if err := saveUploadedFileChecked(fileHeader, targetPath, h.cfg.MaxUploadBytes); err != nil {
		errorResponse(c, http.StatusInternalServerError, "store uploaded asset", err)
		return
	}

	tags := splitAndTrim(c.PostForm("tags"))
	metadata := buildLibraryMetadataFromForm(c, true)
	if len(tags) > 0 {
		metadata["tags"] = tags
	}

	assetType := firstNonEmpty(strings.TrimSpace(c.PostForm("type")), inferAssetType(fileHeader.Filename, c.PostForm("mime_type")))
	item, err := h.store.CreateAsset(c.Request.Context(), Asset{
		AdventureID: stringPtrOrNil(c.PostForm("adventure_id")),
		Type:        assetType,
		SourceType:  firstNonEmpty(strings.TrimSpace(c.PostForm("source_type")), "upload"),
		Name:        firstNonEmpty(strings.TrimSpace(c.PostForm("name")), fileHeader.Filename),
		FilePath:    targetPath,
		MimeType:    firstNonEmpty(strings.TrimSpace(c.PostForm("mime_type")), detectMimeTypeByFilename(fileHeader.Filename)),
		Tags:        tags,
		Metadata:    metadata,
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create asset record", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) importAdventureZip(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "missing zip file", err)
		return
	}

	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file must be a zip archive"})
		return
	}
	if err := ensureUploadSize(fileHeader, h.cfg.MaxZipUploadBytes); err != nil {
		errorResponse(c, uploadErrorStatus(err), "zip archive exceeds allowed size", err)
		return
	}

	adventureID := strings.TrimSpace(c.PostForm("adventure_id"))
	var adventure Adventure
	if adventureID != "" {
		adventures, err := h.store.ListAdventures(c.Request.Context())
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "list adventures", err)
			return
		}

		found := false
		for _, item := range adventures {
			if item.ID == adventureID {
				adventure = item
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "adventure not found"})
			return
		}
	} else {
		adventureName := c.PostForm("adventure_name")
		if adventureName == "" {
			adventureName = strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
		}

		adventure, err = h.store.CreateAdventure(c.Request.Context(), CreateAdventureRequest{
			CampaignID:  stringPtrOrNil(c.PostForm("campaign_id")),
			Name:        adventureName,
			Description: c.PostForm("description"),
			Language:    firstNonEmpty(c.PostForm("language"), "de"),
		})
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "create adventure", err)
			return
		}
	}

	importDir := filepath.Join(h.uploadsDir, "zip-uploads")
	if err := os.MkdirAll(importDir, 0o755); err != nil {
		errorResponse(c, http.StatusInternalServerError, "prepare zip uploads directory", err)
		return
	}

	targetPath := filepath.Join(importDir, fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), sanitizeFilename(fileHeader.Filename)))
	if err := saveUploadedFileChecked(fileHeader, targetPath, h.cfg.MaxZipUploadBytes); err != nil {
		errorResponse(c, http.StatusInternalServerError, "store zip upload", err)
		return
	}
	if err := validateZipArchive(targetPath, h.cfg.MaxZipEntries, h.cfg.MaxZipExtractBytes); err != nil {
		errorResponse(c, uploadErrorStatus(err), "validate zip archive", err)
		return
	}

	report, err := importAdventureZip(c.Request.Context(), h.store, h.uploadsDir, targetPath, adventure, h.cfg.MaxZipEntries, h.cfg.MaxZipExtractBytes)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "import zip adventure", err)
		return
	}

	c.JSON(http.StatusCreated, report)
}

func validDocumentType(value string) bool {
	switch value {
	case "rules", "adventure", "character_sheet", "asset":
		return true
	default:
		return false
	}
}

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, " ", "_")
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, base)
	if base == "" {
		return "upload.bin"
	}
	return base
}

func (h *Handler) persistUploadedDocument(c *gin.Context, fileHeader *multipart.FileHeader, req CreateDocumentRequest) (Document, error) {
	safeName := sanitizeFilename(fileHeader.Filename)
	targetName := fmt.Sprintf("%d-%s", time.Now().UTC().UnixNano(), safeName)
	targetPath := filepath.Join(h.uploadsDir, targetName)
	if err := saveUploadedFileChecked(fileHeader, targetPath, h.cfg.MaxUploadBytes); err != nil {
		return Document{}, err
	}
	return h.persistStoredDocument(c.Request.Context(), targetPath, req)
}

func (h *Handler) persistStoredDocument(ctx context.Context, targetPath string, req CreateDocumentRequest) (Document, error) {
	req.SourceFilePath = &targetPath
	item, err := h.store.CreateDocument(ctx, req)
	if err != nil {
		return Document{}, err
	}

	text, err := extractDocumentText(targetPath)
	if err == nil {
		chunks := chunkDocumentText(text, 1200)
		if len(chunks) > 0 {
			if err := h.store.ReplaceDocumentChunks(ctx, item.ID, chunks, req.Metadata); err == nil {
				item.ChunkCount = len(chunks)
			}
			if refs := extractMonsterReferences(item.Name, chunks); len(refs) > 0 {
				_ = h.store.ReplaceMonsterReferences(ctx, item.ID, refs)
			}
			if entries := extractRuleIndexEntries(item.Name, chunks); len(entries) > 0 {
				_ = h.store.ReplaceRuleIndexEntries(ctx, item.ID, entries)
			}
		}
	}

	return item, nil
}

func buildLibraryMetadataFromForm(c *gin.Context, includePrimaryRuleset bool) map[string]any {
	metadata := map[string]any{}
	rulesetWork := strings.TrimSpace(c.PostForm("ruleset_work"))
	rulesetVersion := strings.TrimSpace(c.PostForm("ruleset_version"))
	compatibleRulesets := splitAndTrim(c.PostForm("compatible_rulesets"))
	adventureIDs := splitAndTrim(c.PostForm("adventure_ids"))

	if includePrimaryRuleset && rulesetWork != "" {
		metadata["ruleset_work"] = rulesetWork
	}
	if includePrimaryRuleset && rulesetVersion != "" {
		metadata["ruleset_version"] = rulesetVersion
	}
	if rulesetWork != "" || rulesetVersion != "" {
		metadata["ruleset_keys"] = []string{fmt.Sprintf("%s:%s", firstNonEmpty(rulesetWork, "default"), firstNonEmpty(rulesetVersion, "default"))}
	}
	if len(compatibleRulesets) > 0 {
		metadata["compatible_rulesets"] = compatibleRulesets
	}
	if len(adventureIDs) > 0 {
		metadata["adventure_ids"] = adventureIDs
	}
	return metadata
}

func splitAndTrim(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func inferAssetType(filename string, mimeType string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.Contains(lower, "token"):
		return "token"
	case strings.Contains(lower, "map"):
		return "map"
	case strings.Contains(lower, "battle"):
		return "battlemap"
	case strings.Contains(lower, "portrait"):
		return "portrait"
	case strings.Contains(lower, "handout"):
		return "handout"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	default:
		return "asset"
	}
}

func detectMimeTypeByFilename(filename string) string {
	if mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

func removeLocalFile(path *string) {
	if path == nil || strings.TrimSpace(*path) == "" {
		return
	}
	_ = os.Remove(*path)
}
