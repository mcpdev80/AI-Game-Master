package httpapi

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const fungalCavernsDemoID = "fungal-caverns-v1"

//go:embed demo_content/fungal-caverns/*
var fungalCavernsDemoFiles embed.FS

func (h *Handler) createFungalCavernsDemo(c *gin.Context) {
	var req CreateFungalCavernsDemoRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid demo payload", err)
			return
		}
	}
	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language != "de" {
		language = "en"
	}

	response, err := h.seedFungalCavernsDemo(c.Request.Context(), language)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "create Fungal Caverns demo", err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) resetFungalCavernsDemo(c *gin.Context) {
	response, err := h.deleteFungalCavernsDemo(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "reset Fungal Caverns demo", err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) seedFungalCavernsDemo(ctx context.Context, language string) (FungalCavernsDemoResponse, error) {
	campaign, campaignReused, err := h.ensureFungalDemoCampaign(ctx)
	if err != nil {
		return FungalCavernsDemoResponse{}, err
	}
	adventure, adventureReused, err := h.ensureFungalDemoAdventure(ctx, campaign)
	if err != nil {
		return FungalCavernsDemoResponse{}, err
	}

	demoDir := filepath.Join(h.uploadsDir, "demo", fungalCavernsDemoID)
	if err := os.MkdirAll(demoDir, 0o755); err != nil {
		return FungalCavernsDemoResponse{}, fmt.Errorf("create demo upload directory: %w", err)
	}
	mapPath, err := writeEmbeddedDemoFile(demoDir, "map-player.png")
	if err != nil {
		return FungalCavernsDemoResponse{}, err
	}
	pdfPath, err := writeEmbeddedDemoFile(demoDir, "original-gm.pdf")
	if err != nil {
		return FungalCavernsDemoResponse{}, err
	}

	if err := h.ensureFungalDemoDocuments(ctx, adventure, pdfPath); err != nil {
		return FungalCavernsDemoResponse{}, err
	}
	mapAsset, assetReused, err := h.ensureFungalDemoMap(ctx, adventure, mapPath)
	if err != nil {
		return FungalCavernsDemoResponse{}, err
	}
	if err := h.ensureFungalDemoCharacter(ctx, campaign); err != nil {
		return FungalCavernsDemoResponse{}, err
	}
	session, sessionReused, err := h.ensureFungalDemoSession(ctx, campaign, adventure, mapAsset, language)
	if err != nil {
		return FungalCavernsDemoResponse{}, err
	}

	return FungalCavernsDemoResponse{
		Campaign:        campaign,
		Adventure:       adventure,
		Session:         session,
		MapAsset:        mapAsset,
		GMURL:           "/sessions/" + session.ID,
		PlayerScreenURL: "/player-screen",
		Reused:          campaignReused && adventureReused && assetReused && sessionReused,
	}, nil
}

func (h *Handler) ensureFungalDemoCampaign(ctx context.Context) (Campaign, bool, error) {
	items, err := h.store.ListCampaigns(ctx)
	if err != nil {
		return Campaign{}, false, err
	}
	for _, item := range items {
		if item.Name == "Build Week Demo — The Fungal Caverns" {
			return item, true, nil
		}
	}
	item, err := h.store.CreateCampaign(ctx, CreateCampaignRequest{
		Name:        "Build Week Demo — The Fungal Caverns",
		Description: "Bilingual, system-neutral AI Game Master demo. Source adventure and art by Logen Nein, CC BY 3.0 US.",
	})
	return item, false, err
}

func (h *Handler) ensureFungalDemoAdventure(ctx context.Context, campaign Campaign) (Adventure, bool, error) {
	items, err := h.store.ListAdventures(ctx)
	if err != nil {
		return Adventure{}, false, err
	}
	for _, item := range items {
		if metadataString(item.Metadata, "demo_id") == fungalCavernsDemoID {
			return item, true, nil
		}
	}
	item, err := h.store.CreateAdventure(ctx, CreateAdventureRequest{
		CampaignID:  &campaign.ID,
		Name:        "The Fungal Caverns / Die Pilzhöhlen",
		Description: "A bilingual, system-neutral cave expedition with an AI-directed player map.",
		Language:    "en,de",
		Metadata: map[string]any{
			"demo_id":     fungalCavernsDemoID,
			"creator":     "Logen Nein",
			"license":     "CC BY 3.0 US",
			"source_url":  "https://logen-nein.itch.io/the-fungal-caverns",
			"license_url": "https://creativecommons.org/licenses/by/3.0/us/",
		},
	})
	return item, false, err
}

func (h *Handler) ensureFungalDemoDocuments(ctx context.Context, adventure Adventure, pdfPath string) error {
	existing, err := h.store.ListDocuments(ctx)
	if err != nil {
		return err
	}
	byKind := map[string]Document{}
	for _, document := range existing {
		if metadataString(document.Metadata, "demo_id") == fungalCavernsDemoID {
			byKind[metadataString(document.Metadata, "demo_kind")] = document
		}
	}

	type demoDocument struct {
		kind, name, filename, language string
	}
	documents := []demoDocument{
		{kind: "guide_en", name: "The Fungal Caverns — AI GM Guide (EN)", filename: "adventure-en.md", language: "en"},
		{kind: "guide_de", name: "Die Pilzhöhlen — AI-GM-Leitfaden (DE)", filename: "adventure-de.md", language: "de"},
		{kind: "attribution", name: "The Fungal Caverns — Attribution & License", filename: "ATTRIBUTION.md", language: "en"},
	}
	for _, spec := range documents {
		content, readErr := fungalCavernsDemoFiles.ReadFile("demo_content/fungal-caverns/" + spec.filename)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", spec.filename, readErr)
		}
		document, ok := byKind[spec.kind]
		if !ok {
			document, err = h.store.CreateDocument(ctx, CreateDocumentRequest{
				AdventureID: &adventure.ID,
				Type:        "adventure",
				Name:        spec.name,
				Metadata: map[string]any{
					"demo_id": fungalCavernsDemoID, "demo_kind": spec.kind, "language": spec.language,
					"license": "CC BY 3.0 US", "creator": "Logen Nein",
					"embedded_content": string(content), "embedded_content_type": "text/markdown; charset=utf-8",
				},
			})
			if err != nil {
				return err
			}
		}
		if spec.kind != "attribution" {
			if err := h.store.ReplaceDocumentChunks(ctx, document.ID, chunkDocumentText(string(content), 1200), map[string]any{"language": spec.language, "demo_id": fungalCavernsDemoID}); err != nil {
				return err
			}
		}
	}

	if _, ok := byKind["original_pdf"]; !ok {
		_, err = h.store.CreateDocument(ctx, CreateDocumentRequest{
			AdventureID:    &adventure.ID,
			Type:           "adventure",
			Name:           "The Fungal Caverns — Original GM One-Page PDF",
			SourceFilePath: &pdfPath,
			Metadata:       map[string]any{"demo_id": fungalCavernsDemoID, "demo_kind": "original_pdf", "creator": "Logen Nein", "license": "CC BY 3.0 US"},
		})
	}
	return err
}

func (h *Handler) ensureFungalDemoMap(ctx context.Context, adventure Adventure, mapPath string) (Asset, bool, error) {
	assets, err := h.store.ListAssets(ctx)
	if err != nil {
		return Asset{}, false, err
	}
	for _, asset := range assets {
		if metadataString(asset.Metadata, "demo_id") == fungalCavernsDemoID && metadataString(asset.Metadata, "cue_key") == "fungal_caverns_map" {
			return asset, true, nil
		}
	}
	location := "The Fungal Caverns / Die Pilzhöhlen"
	asset, err := h.store.CreateAsset(ctx, Asset{
		AdventureID: &adventure.ID, Type: "map", SourceType: "licensed_demo", Name: "The Fungal Caverns — Player Map",
		FilePath: mapPath, MimeType: "image/png", LocationName: &location,
		Tags: []string{"fungal_caverns_map", "cavern", "map", "player-safe", "fungal caverns", "pilzhöhlen"},
		Metadata: map[string]any{
			"demo_id": fungalCavernsDemoID, "cue_key": "fungal_caverns_map", "creator": "Logen Nein", "license": "CC BY 3.0 US",
			"source_url": "https://logen-nein.itch.io/the-fungal-caverns", "player_safe": true,
		},
	})
	return asset, false, err
}

func (h *Handler) ensureFungalDemoCharacter(ctx context.Context, campaign Campaign) error {
	characters, err := h.store.ListCharacters(ctx)
	if err != nil {
		return err
	}
	for _, character := range characters {
		if metadataString(character.Metadata, "demo_id") == fungalCavernsDemoID {
			return nil
		}
	}
	armorClass, hitPoints := 13, 10
	_, err = h.store.CreateCharacter(ctx, Character{
		CampaignID: &campaign.ID, Name: "Rowan", PlayerName: "Demo Player", ClassAndLevel: "Scout 1", Background: "Cave Cartographer",
		Race: "Human", Alignment: "Curious", ArmorClass: &armorClass, Speed: "30 ft", HitPointMax: &hitPoints, Proficiency: "+2",
		Abilities: map[string]int{"strength": 10, "dexterity": 15, "constitution": 12, "intelligence": 14, "wisdom": 13, "charisma": 8},
		Languages: []string{"Common / Gemeinsprache"}, Features: []string{"Keen observer / Aufmerksame Beobachtung", "Cartographer / Kartografie"},
		Metadata: map[string]any{"demo_id": fungalCavernsDemoID, "system_neutral": true},
	})
	return err
}

func (h *Handler) ensureFungalDemoSession(ctx context.Context, campaign Campaign, adventure Adventure, mapAsset Asset, language string) (Session, bool, error) {
	sessions, err := h.store.ListSessions(ctx)
	if err != nil {
		return Session{}, false, err
	}
	for _, session := range sessions {
		if session.AdventureID != nil && *session.AdventureID == adventure.ID && strings.HasPrefix(session.Name, "Fungal Caverns Demo") {
			return session, true, nil
		}
	}
	session, err := h.store.CreateSession(ctx, CreateSessionRequest{
		CampaignID: campaign.ID, Name: "Fungal Caverns Demo — EN / DE", AdventureID: &adventure.ID,
		RulesetWork: "System Neutral", RulesetVersion: "1.0", TargetPlayerCount: 1, Status: "live",
		CurrentScene: "The sheltered entrance / Der geschützte Eingang", CurrentLocation: "Cavern entrance", Language: language,
	})
	if err != nil {
		return Session{}, false, err
	}
	opening := "A sheltered cave waits beyond the rain. A massive boulder hides narrow cracks leading into darkness. What do you do?"
	if language == "de" {
		opening = "Hinter dem Regen liegt eine geschützte Höhle. Ein massiver Felsblock verbirgt enge Spalten in die Dunkelheit. Was tut ihr?"
	}
	session.State.SceneSummary = opening
	session.State.LastNarration = opening
	session.State.SessionRecap = opening
	session.State.ActiveMediaCue = "fungal_caverns_map"
	session.State.VisualMode = "scene"
	session.State.VisualPayload = map[string]any{
		"type": "scene_image", "title": "The Fungal Caverns / Die Pilzhöhlen", "scene": opening,
		"narration": opening, "image_asset_id": mapAsset.ID, "image_cue": "fungal_caverns_map",
	}
	if err := h.store.UpdateSessionState(ctx, session.ID, session.State); err != nil {
		return Session{}, false, err
	}
	return session, false, nil
}

func (h *Handler) deleteFungalCavernsDemo(ctx context.Context) (ResetFungalCavernsDemoResponse, error) {
	response := ResetFungalCavernsDemoResponse{Deleted: true}

	sessions, err := h.store.ListSessions(ctx)
	if err != nil {
		return response, err
	}
	for _, session := range sessions {
		if !isFungalDemoSession(session) {
			continue
		}
		if response.AdventureID == "" && session.AdventureID != nil {
			response.AdventureID = strings.TrimSpace(*session.AdventureID)
		}
		deleted, deleteErr := h.store.DeleteLLMSessionsByScope(ctx, "session", session.ID)
		if deleteErr != nil {
			return response, deleteErr
		}
		response.ArchivedLLMCount += int(deleted)
		if err := h.store.DeleteSession(ctx, session.ID); err != nil {
			return response, err
		}
		response.SessionCount++
	}

	characters, err := h.store.ListCharacters(ctx)
	if err != nil {
		return response, err
	}
	for _, character := range characters {
		if !isFungalDemoCharacter(character) {
			continue
		}
		if response.CampaignID == "" && character.CampaignID != nil {
			response.CampaignID = strings.TrimSpace(*character.CampaignID)
		}
		if err := h.store.DeleteCharacter(ctx, character.ID); err != nil {
			return response, err
		}
		response.CharacterCount++
	}

	assets, err := h.store.ListAssets(ctx)
	if err != nil {
		return response, err
	}
	for _, asset := range assets {
		if !isFungalDemoAsset(asset) {
			continue
		}
		if response.AdventureID == "" && asset.AdventureID != nil {
			response.AdventureID = strings.TrimSpace(*asset.AdventureID)
		}
		filePath := asset.FilePath
		if err := h.store.DeleteAsset(ctx, asset.ID); err != nil {
			return response, err
		}
		removeLocalFile(&filePath)
		response.AssetCount++
	}

	documents, err := h.store.ListDocuments(ctx)
	if err != nil {
		return response, err
	}
	for _, document := range documents {
		if !isFungalDemoDocument(document) {
			continue
		}
		if response.AdventureID == "" && document.AdventureID != nil {
			response.AdventureID = strings.TrimSpace(*document.AdventureID)
		}
		if err := h.store.DeleteDocument(ctx, document.ID); err != nil {
			return response, err
		}
		removeLocalFile(document.SourceFilePath)
		response.DocumentCount++
	}

	adventures, err := h.store.ListAdventures(ctx)
	if err != nil {
		return response, err
	}
	for _, adventure := range adventures {
		if !isFungalDemoAdventure(adventure) {
			continue
		}
		if response.AdventureID == "" {
			response.AdventureID = adventure.ID
		}
		if response.CampaignID == "" && adventure.CampaignID != nil {
			response.CampaignID = strings.TrimSpace(*adventure.CampaignID)
		}
		if err := h.store.DeleteAdventure(ctx, adventure.ID); err != nil {
			return response, err
		}
	}

	campaigns, err := h.store.ListCampaigns(ctx)
	if err != nil {
		return response, err
	}
	for _, campaign := range campaigns {
		if !isFungalDemoCampaign(campaign) {
			continue
		}
		if response.CampaignID == "" {
			response.CampaignID = campaign.ID
		}
		if err := h.store.DeleteCampaign(ctx, campaign.ID); err != nil {
			return response, err
		}
	}

	removeDemoUploadDirectory(filepath.Join(h.uploadsDir, "demo", fungalCavernsDemoID))
	return response, nil
}

func writeEmbeddedDemoFile(directory, name string) (string, error) {
	content, err := fungalCavernsDemoFiles.ReadFile("demo_content/fungal-caverns/" + name)
	if err != nil {
		return "", fmt.Errorf("read embedded %s: %w", name, err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("write demo file %s: %w", name, err)
	}
	return path, nil
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func isFungalDemoCampaign(campaign Campaign) bool {
	return strings.TrimSpace(campaign.Name) == "Build Week Demo — The Fungal Caverns"
}

func isFungalDemoAdventure(adventure Adventure) bool {
	return metadataString(adventure.Metadata, "demo_id") == fungalCavernsDemoID
}

func isFungalDemoSession(session Session) bool {
	if session.AdventureID != nil && strings.TrimSpace(*session.AdventureID) != "" {
		return strings.HasPrefix(strings.TrimSpace(session.Name), "Fungal Caverns Demo")
	}
	return false
}

func isFungalDemoCharacter(character Character) bool {
	return metadataString(character.Metadata, "demo_id") == fungalCavernsDemoID
}

func isFungalDemoDocument(document Document) bool {
	return metadataString(document.Metadata, "demo_id") == fungalCavernsDemoID
}

func isFungalDemoAsset(asset Asset) bool {
	return metadataString(asset.Metadata, "demo_id") == fungalCavernsDemoID
}

func removeDemoUploadDirectory(path string) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return
	}
	_ = os.RemoveAll(trimmed)
}
