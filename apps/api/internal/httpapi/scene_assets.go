package httpapi

import (
	"context"
	"strings"
	"unicode"
)

func (h *Handler) resolveAdventureSceneAsset(ctx context.Context, adventureID string, events []SceneEvent) (*Asset, error) {
	assets, err := h.store.ListAssets(ctx)
	if err != nil {
		return nil, err
	}
	cues := make([]string, 0, len(events))
	for _, event := range events {
		if cue := normalizeAssetCue(event.Name); cue != "" {
			cues = append(cues, cue)
		}
	}

	var best *Asset
	bestScore := -1
	for index := range assets {
		asset := &assets[index]
		if asset.AdventureID == nil || *asset.AdventureID != adventureID || !strings.HasPrefix(strings.ToLower(asset.MimeType), "image/") {
			continue
		}
		score := sceneAssetScore(*asset, cues)
		if score > bestScore {
			copyOfAsset := *asset
			best = &copyOfAsset
			bestScore = score
		}
	}
	return best, nil
}

func sceneAssetScore(asset Asset, cues []string) int {
	values := []string{asset.Name, asset.Type, metadataString(asset.Metadata, "cue_key")}
	if asset.LocationName != nil {
		values = append(values, *asset.LocationName)
	}
	values = append(values, asset.Tags...)
	normalizedValues := make([]string, 0, len(values))
	for _, value := range values {
		if normalized := normalizeAssetCue(value); normalized != "" {
			normalizedValues = append(normalizedValues, normalized)
		}
	}
	score := 0
	if strings.EqualFold(asset.Type, "map") {
		score = 10
	}
	for _, cue := range cues {
		for _, value := range normalizedValues {
			switch {
			case cue == value:
				score += 100
			case strings.Contains(value, cue) || strings.Contains(cue, value):
				score += 35
			}
		}
	}
	return score
}

func normalizeAssetCue(value string) string {
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '_'
	}, strings.TrimSpace(value))
	return strings.Join(strings.FieldsFunc(mapped, func(r rune) bool { return r == '_' }), "_")
}
