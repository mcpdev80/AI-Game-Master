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
	targetCharacters := fungalDemoCharacters(campaign.ID)
	targetByName := map[string]Character{}
	targetNames := map[string]struct{}{}
	for _, character := range targetCharacters {
		name := strings.TrimSpace(character.Name)
		targetNames[name] = struct{}{}
		targetByName[name] = character
	}
	existingByName := map[string]Character{}
	for _, character := range characters {
		if metadataString(character.Metadata, "demo_id") == fungalCavernsDemoID {
			name := strings.TrimSpace(character.Name)
			if _, ok := targetNames[name]; !ok {
				if err := h.store.DeleteCharacter(ctx, character.ID); err != nil {
					return err
				}
				continue
			}
			existingByName[name] = character
		}
	}
	for _, character := range targetCharacters {
		name := strings.TrimSpace(character.Name)
		existing, ok := existingByName[name]
		if ok {
			replacement := targetByName[name]
			replacement.ID = existing.ID
			replacement.CreatedAt = existing.CreatedAt
			if _, err := h.store.UpdateCharacter(ctx, replacement); err != nil {
				return err
			}
			continue
		}
		if _, err := h.store.CreateCharacter(ctx, character); err != nil {
			return err
		}
	}
	return nil
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

func fungalDemoCharacters(campaignID string) []Character {
	baseMetadata := func(extra map[string]any) map[string]any {
		metadata := map[string]any{
			"demo_id":                    fungalCavernsDemoID,
			"demo_character":             true,
			"rules_profile":              "5E SRD 5.1",
			"current_hit_points":         "",
			"temporary_hit_points":       "0",
			"current_money":              "",
			"experience_points":          "0",
			"level_up_available":         "No",
			"session_notes":              []string{},
			"allies":                     "The other demo adventurers in the Fungal Caverns party.",
			"senses":                     "Passive Perception 10",
			"size":                       "Medium",
			"weight":                     "",
			"age":                        "",
			"eyes":                       "",
			"skin":                       "",
			"hair":                       "",
			"spell_attacks":              "",
			"spell_notes":                "",
			"spells":                     []string{},
			"starting_money":             "",
			"starting_equipment":         []string{},
			"current_inventory":          []string{},
			"tools_and_proficiencies":    []string{},
			"skill_proficiencies":        []string{},
			"saving_throw_proficiencies": []string{},
			"weapon_notes":               []string{},
			"combat_attacks":             "",
			"combat_overview":            "",
			"concept":                    "",
			"backstory":                  "",
			"personality_traits":         "",
			"ideals":                     "",
			"bonds":                      "",
			"flaws":                      "",
			"hit_dice":                   "",
			"passive_perception":         "",
		}
		for key, value := range extra {
			metadata[key] = value
		}
		return metadata
	}

	return []Character{
		{
			CampaignID:    &campaignID,
			Name:          "Seraphine Vale",
			PlayerName:    "Demo Player",
			ClassAndLevel: "Paladin 1",
			Background:    "Acolyte",
			Race:          "Human",
			Alignment:     "Lawful Good",
			ArmorClass:    demoIntPtr(18),
			Speed:         "30 ft",
			HitPointMax:   demoIntPtr(12),
			Proficiency:   "+2",
			Abilities:     map[string]int{"strength": 16, "dexterity": 10, "constitution": 14, "intelligence": 10, "wisdom": 12, "charisma": 14},
			Languages:     []string{"Common", "Celestial"},
			Features:      []string{"Divine Sense", "Lay on Hands", "Holy Resolve"},
			Metadata: baseMetadata(map[string]any{
				"current_hit_points":         "12",
				"current_money":              "15 gp",
				"age":                        "24",
				"weight":                     "180 lb",
				"eyes":                       "Steel gray",
				"skin":                       "Fair",
				"hair":                       "Black",
				"hit_dice":                   "1d10",
				"passive_perception":         "11",
				"concept":                    "A steadfast frontline guardian built to protect the group and stabilize a fight.",
				"backstory":                  "Seraphine was raised in a monastery hospice and took sacred vows after surviving a border raid. She now travels to stop corruption before it spreads.",
				"personality_traits":         "Calm under pressure; direct; quietly compassionate.",
				"ideals":                     "Duty. The strong must shield those who cannot shield themselves.",
				"bonds":                      "She carries a sun-marked prayer strip from the abbey where she trained.",
				"flaws":                      "She is slow to trust people who hide the truth.",
				"skill_proficiencies":        []string{"Athletics", "Insight", "Religion", "Persuasion"},
				"saving_throw_proficiencies": []string{"Wisdom", "Charisma"},
				"tools_and_proficiencies":    []string{"Herbalism kit"},
				"starting_equipment":         []string{"Chain mail", "Shield", "Longsword", "5 javelins", "Priest's pack", "Holy symbol"},
				"current_inventory":          []string{"Chain mail", "Shield", "Longsword", "5 javelins", "Holy symbol", "Bedroll", "Rations (5 days)"},
				"weapon_notes":               []string{"Use the longsword and shield in melee.", "Open with a javelin throw if enemies are closing at range."},
				"combat_attacks":             "Longsword | +2 | STR | Melee | +5 | 1d8+3 | Slashing\nDescription: One-handed martial melee weapon used with shield.\nJavelin | +2 | STR | 30/120 ft | +5 | 1d6+3 | Piercing\nDescription: Thrown opening attack before enemies close.",
				"combat_overview":            "Frontline defender with strong AC, emergency healing from Lay on Hands, and solid melee accuracy.",
			}),
		},
		{
			CampaignID:    &campaignID,
			Name:          "Rowan Quickstep",
			PlayerName:    "Demo Player",
			ClassAndLevel: "Rogue 1",
			Background:    "Criminal",
			Race:          "Lightfoot Halfling",
			Alignment:     "Chaotic Good",
			ArmorClass:    demoIntPtr(15),
			Speed:         "25 ft",
			HitPointMax:   demoIntPtr(9),
			Proficiency:   "+2",
			Abilities:     map[string]int{"strength": 8, "dexterity": 17, "constitution": 13, "intelligence": 12, "wisdom": 14, "charisma": 12},
			Languages:     []string{"Common", "Halfling", "Thieves' Cant"},
			Features:      []string{"Sneak Attack", "Expertise", "Halfling Nimbleness"},
			Metadata: baseMetadata(map[string]any{
				"current_hit_points":         "9",
				"current_money":              "12 gp, 8 sp",
				"age":                        "22",
				"weight":                     "38 lb",
				"eyes":                       "Hazel",
				"skin":                       "Tan",
				"hair":                       "Chestnut",
				"hit_dice":                   "1d8",
				"passive_perception":         "14",
				"concept":                    "A fast scout and skill expert who opens locks, spots danger, and lands precise ranged hits.",
				"backstory":                  "Rowan grew up running messages and contraband through crowded river wards. A close call with a gang boss convinced him to put his talents to better use.",
				"personality_traits":         "Restless, observant, and impossible to intimidate for long.",
				"ideals":                     "Freedom. No tyrant, lock, or lie should control the road ahead.",
				"bonds":                      "He still sends coin to his younger sister whenever he can.",
				"flaws":                      "He pushes his luck because he hates backing down from a challenge.",
				"skill_proficiencies":        []string{"Acrobatics", "Perception", "Sleight of Hand", "Stealth", "Investigation", "Deception"},
				"saving_throw_proficiencies": []string{"Dexterity", "Intelligence"},
				"tools_and_proficiencies":    []string{"Thieves' tools", "Disguise kit"},
				"starting_equipment":         []string{"Leather armor", "Rapier", "Shortbow", "20 arrows", "2 daggers", "Thieves' tools", "Burglar's pack"},
				"current_inventory":          []string{"Leather armor", "Rapier", "Shortbow", "20 arrows", "2 daggers", "Thieves' tools", "Crowbar", "Hooded lantern"},
				"weapon_notes":               []string{"Aim for advantage to trigger Sneak Attack.", "Stay mobile and avoid ending a turn exposed in the front line."},
				"combat_attacks":             "Rapier | +2 | DEX | Melee | +5 | 1d8+3 | Piercing\nDescription: Finesse melee attack for close pressure.\nShortbow | +2 | DEX | 80/320 ft | +5 | 1d6+3 | Piercing\nDescription: Preferred ranged attack for setting up Sneak Attack.",
				"combat_overview":            "Scout, trap-handler, and precision striker. Best when attacking with advantage or beside an ally engaging the same target.",
			}),
		},
		{
			CampaignID:    &campaignID,
			Name:          "Brother Alden",
			PlayerName:    "Demo Player",
			ClassAndLevel: "Cleric 1",
			Background:    "Hermit",
			Race:          "Hill Dwarf",
			Alignment:     "Neutral Good",
			ArmorClass:    demoIntPtr(18),
			Speed:         "25 ft",
			HitPointMax:   demoIntPtr(11),
			Proficiency:   "+2",
			Abilities:     map[string]int{"strength": 14, "dexterity": 10, "constitution": 14, "intelligence": 10, "wisdom": 16, "charisma": 12},
			Languages:     []string{"Common", "Dwarvish"},
			Features:      []string{"Spellcasting", "Life Domain", "Disciple of Life"},
			Metadata: baseMetadata(map[string]any{
				"current_hit_points":         "11",
				"current_money":              "10 gp",
				"age":                        "58",
				"weight":                     "162 lb",
				"eyes":                       "Blue",
				"skin":                       "Ruddy",
				"hair":                       "Brown with silver braids",
				"hit_dice":                   "1d8",
				"passive_perception":         "13",
				"concept":                    "A sturdy healer and support caster who can also hold a doorway in armor.",
				"backstory":                  "Brother Alden spent years in a mountain shrine preserving old records and tending pilgrims. He left isolation after recurring visions of blight spreading underground.",
				"personality_traits":         "Patient, practical, and gently stubborn.",
				"ideals":                     "Mercy. A life saved today can still change the world tomorrow.",
				"bonds":                      "He records the names of everyone he fails to save and prays for them at dawn.",
				"flaws":                      "He will overextend himself before admitting exhaustion.",
				"skill_proficiencies":        []string{"Insight", "Medicine", "Religion", "Survival"},
				"saving_throw_proficiencies": []string{"Wisdom", "Charisma"},
				"tools_and_proficiencies":    []string{"Herbalism kit", "Brewer's supplies"},
				"starting_equipment":         []string{"Chain mail", "Shield", "Mace", "Light crossbow", "20 bolts", "Priest's pack", "Holy symbol"},
				"current_inventory":          []string{"Chain mail", "Shield", "Mace", "Light crossbow", "20 bolts", "Holy symbol", "Healer's kit", "Rations (5 days)"},
				"weapon_notes":               []string{"Hold the line with shield and mace.", "Use Sacred Flame when melee would be unsafe."},
				"combat_attacks":             "Mace | +2 | STR | Melee | +4 | 1d6+2 | Bludgeoning\nDescription: Reliable close-range backup weapon.\nLight Crossbow | +2 | DEX | 80/320 ft | +2 | 1d8 | Piercing\nDescription: Ranged option when staying behind the front line.",
				"spell_save_dc":              "13",
				"spell_attack_bonus":         "+5",
				"spell_attacks":              "Cantrip | Sacred Flame | WIS | 60 ft | DC 13 | 1d8 | Radiant\nDescription: Target makes a Dexterity save instead of an attack roll.\n1st | Guiding Bolt | WIS | 120 ft | +5 | 4d6 | Radiant\nDescription: Strong opening spell that grants advantage on the next attack against the target.",
				"spell_notes":                "Bless: Up to three allies add 1d4 to attacks and saving throws while you maintain concentration.\nCure Wounds: Touch heal for 1d8 + Wisdom modifier.\nHealing Word: Bonus-action heal at 60 ft for emergency recovery.\nSanctuary: Wards an ally so attackers must pass a Wisdom save to target them.\nGuidance: Add 1d4 to one ability check before the roll.\nLight: Makes an object shine like a torch.\nSacred Flame: Radiant cantrip that forces a Dexterity save.\nGuiding Bolt: Heavy radiant hit that grants advantage to the next attacker.",
				"spells":                     []string{"Guidance", "Light", "Sacred Flame", "Bless", "Cure Wounds", "Healing Word", "Guiding Bolt", "Sanctuary"},
				"combat_overview":            "Primary healer and support caster. Strong opening turns are Bless, Guiding Bolt, or Healing Word to recover a fallen ally.",
			}),
		},
		{
			CampaignID:    &campaignID,
			Name:          "Elira Moonfall",
			PlayerName:    "Demo Player",
			ClassAndLevel: "Wizard 1",
			Background:    "Sage",
			Race:          "High Elf",
			Alignment:     "Neutral",
			ArmorClass:    demoIntPtr(12),
			Speed:         "30 ft",
			HitPointMax:   demoIntPtr(7),
			Proficiency:   "+2",
			Abilities:     map[string]int{"strength": 8, "dexterity": 14, "constitution": 13, "intelligence": 16, "wisdom": 12, "charisma": 10},
			Languages:     []string{"Common", "Elvish", "Draconic"},
			Features:      []string{"Spellcasting", "Arcane Recovery", "Cantrip Training"},
			Metadata: baseMetadata(map[string]any{
				"current_hit_points":         "7",
				"current_money":              "8 gp",
				"age":                        "121",
				"weight":                     "118 lb",
				"eyes":                       "Violet",
				"skin":                       "Pale bronze",
				"hair":                       "Silver-blond",
				"hit_dice":                   "1d6",
				"passive_perception":         "11",
				"concept":                    "A battlefield controller and utility caster with strong ranged cantrips and classic defensive magic.",
				"backstory":                  "Elira apprenticed under a reclusive archivist who taught her to value preparation above bravado. She adventures to recover lore before it is lost or misused.",
				"personality_traits":         "Curious, exacting, and always mentally three steps ahead.",
				"ideals":                     "Knowledge. A hidden truth is still a kind of danger.",
				"bonds":                      "Her master's annotated spellbook margins still guide every difficult decision.",
				"flaws":                      "She can become so focused on the ideal solution that she hesitates in messy situations.",
				"skill_proficiencies":        []string{"Arcana", "History", "Investigation", "Insight"},
				"saving_throw_proficiencies": []string{"Intelligence", "Wisdom"},
				"tools_and_proficiencies":    []string{"Calligrapher's supplies"},
				"starting_equipment":         []string{"Quarterstaff", "Component pouch", "Scholar's pack", "Spellbook"},
				"current_inventory":          []string{"Quarterstaff", "Dagger", "Component pouch", "Spellbook", "Ink and quill", "Scholar's pack"},
				"weapon_notes":               []string{"Open with control or ranged damage before enemies close.", "Cast Mage Armor early if danger is expected."},
				"combat_attacks":             "Quarterstaff | +2 | STR | Melee | +2 | 1d6 | Bludgeoning\nDescription: Emergency melee option only.\nDagger | +2 | DEX | 20/60 ft | +4 | 1d4+2 | Piercing\nDescription: Backup thrown weapon when conserving spell slots.",
				"spell_save_dc":              "13",
				"spell_attack_bonus":         "+5",
				"spell_attacks":              "Cantrip | Fire Bolt | INT | 120 ft | +5 | 1d10 | Fire\nDescription: Primary ranged cantrip for steady damage.\n1st | Magic Missile | INT | 120 ft | Auto | 3d4+3 | Force\nDescription: Automatically hits and is ideal for finishing a wounded target.",
				"spell_notes":                "Mage Armor: Sets base AC to 13 + Dexterity modifier for 8 hours.\nMagic Missile: Three force darts that hit automatically.\nShield: Reaction spell that grants +5 AC until your next turn.\nSleep: Puts creatures to sleep starting with the lowest current HP.\nDetect Magic: Reveals magical auras while you concentrate.\nComprehend Languages: Lets you understand spoken and written language for 1 hour.\nFire Bolt: Long-range fire cantrip for steady damage.\nMage Hand: Creates a spectral hand for simple remote interactions.\nMinor Illusion: Creates a small sound or image for distraction and misdirection.\nLight: Makes an object glow brightly for exploration.",
				"spells":                     []string{"Fire Bolt", "Mage Hand", "Minor Illusion", "Light", "Mage Armor", "Magic Missile", "Shield", "Sleep", "Detect Magic", "Comprehend Languages"},
				"combat_overview":            "Backline caster with flexible utility. Best protected by allies while she controls the pace of a fight.",
			}),
		},
	}
}

func demoIntPtr(value int) *int {
	return &value
}
