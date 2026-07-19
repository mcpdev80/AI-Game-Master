package httpapi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// State-Update-Allowlist
// ---------------------------------------------------------------------------

// Character state-update fields that the model is allowed to modify.
// Keys are the normalised field name; values are aliases that map to the
// canonical name.
var characterUpdateFields = map[string]string{
	"experience_points":  "experience_points",
	"xp":                 "experience_points",
	"ep":                 "experience_points",
	"money":              "money",
	"gold":               "money",
	"current_money":      "money",
	"inventory_add":      "inventory_add",
	"item_add":           "inventory_add",
	"loot_add":           "inventory_add",
	"inventory_remove":   "inventory_remove",
	"item_remove":        "inventory_remove",
	"loot_remove":        "inventory_remove",
	"level_up":           "level_up_available",
	"level_up_available": "level_up_available",
	"level_up_ready":     "level_up_available",
	"notes_add":          "notes_add",
	"character_note":     "notes_add",
}

// sessionUpdateFields maps normalised session-level fields to their canonical name.
var sessionUpdateFields = map[string]string{
	"group_gold":             "group_gold",
	"group_inventory_add":    "group_inventory_add",
	"group_inventory_remove": "group_inventory_remove",
	"group_notes":            "group_notes",
}

// StateUpdateValidationError describes why a single update was rejected.
type StateUpdateValidationError struct {
	EntityID string
	Field    string
	Reason   string
}

func (e *StateUpdateValidationError) Error() string {
	if e.EntityID != "" {
		return fmt.Sprintf("state update %s.%s rejected: %s", e.EntityID, e.Field, e.Reason)
	}
	return fmt.Sprintf("state update %s rejected: %s", e.Field, e.Reason)
}

// validateStateUpdates checks every update against the allowlist and returns
// the list of rejected updates.  Valid updates are returned unchanged; invalid
// ones are omitted from the result.
func validateStateUpdates(updates []StateUpdate, knownCharacterIDs map[string]struct{}) ([]StateUpdate, []StateUpdateValidationError) {
	if len(updates) == 0 {
		return updates, nil
	}

	validated := make([]StateUpdate, 0, len(updates))
	var errors []StateUpdateValidationError

	for _, u := range updates {
		entityID := strings.TrimSpace(u.EntityID)
		if entityID == "" {
			errors = append(errors, StateUpdateValidationError{Field: u.Field, Reason: "entity_id is empty"})
			continue
		}

		// --- entity validation ---
		isSession := strings.EqualFold(entityID, "session")
		isCampaign := strings.EqualFold(entityID, "campaign")

		if isCampaign {
			errors = append(errors, StateUpdateValidationError{EntityID: entityID, Field: u.Field, Reason: "campaign entity is not allowed"})
			continue
		}

		if !isSession {
			// Must reference a known character.
			if _, known := knownCharacterIDs[entityID]; !known {
				errors = append(errors, StateUpdateValidationError{EntityID: entityID, Field: u.Field, Reason: "unknown entity_id"})
				continue
			}
		}

		// --- field validation ---
		normalised := normalizeProgressField(u.Field)
		if isSession {
			if _, ok := sessionUpdateFields[normalised]; !ok {
				errors = append(errors, StateUpdateValidationError{EntityID: entityID, Field: u.Field, Reason: fmt.Sprintf("unknown session field %q", u.Field)})
				continue
			}
		} else {
			if _, ok := characterUpdateFields[normalised]; !ok {
				errors = append(errors, StateUpdateValidationError{EntityID: entityID, Field: u.Field, Reason: fmt.Sprintf("unknown character field %q", u.Field)})
				continue
			}
		}

		// --- value consistency ---
		if err := validateStateUpdateValue(u); err != nil {
			errors = append(errors, StateUpdateValidationError{EntityID: entityID, Field: u.Field, Reason: err.Error()})
			continue
		}

		validated = append(validated, u)
	}

	return validated, errors
}

// validateStateUpdateValue checks value/delta consistency for a single update.
func validateStateUpdateValue(u StateUpdate) error {
	value := strings.TrimSpace(u.Value)
	normalised := normalizeProgressField(u.Field)

	// Fields that carry a delta (numeric).
	deltaFields := map[string]struct{}{
		"experience_points": {},
		"xp":                {},
		"ep":                {},
		"money":             {},
		"gold":              {},
		"current_money":     {},
		"group_gold":        {},
	}

	// Fields that carry a value (string).
	valueFields := map[string]struct{}{
		"inventory_add":          {},
		"item_add":               {},
		"loot_add":               {},
		"inventory_remove":       {},
		"item_remove":            {},
		"loot_remove":            {},
		"group_inventory_add":    {},
		"group_inventory_remove": {},
		"group_notes":            {},
		"notes_add":              {},
		"character_note":         {},
		"level_up":               {},
		"level_up_available":     {},
		"level_up_ready":         {},
	}

	if _, deltaOnly := deltaFields[normalised]; deltaOnly {
		if value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("value must be numeric for field %q", u.Field)
			}
			if parsed < 0 {
				return fmt.Errorf("value must not be negative for field %q", u.Field)
			}
			if u.Delta != 0 {
				return fmt.Errorf("field %q must use either delta or value, not both", u.Field)
			}
		} else if u.Delta == 0 {
			return fmt.Errorf("field %q requires a numeric delta or value", u.Field)
		}
	}

	if _, valOnly := valueFields[normalised]; valOnly {
		if value == "" {
			return fmt.Errorf("value must be non-empty for field %q", u.Field)
		}
		if u.Delta != 0 {
			return fmt.Errorf("field %q does not support delta", u.Field)
		}
	}

	return nil
}

// knownCharacterIDsFromSession returns a map of character IDs that belong to
// the given session (i.e. characters that are actually active).
func knownCharacterIDsFromSession(session Session, characters []Character) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, ch := range characters {
		ids[ch.ID] = struct{}{}
	}
	return ids
}

// ---------------------------------------------------------------------------
// Roll-Request Validation
// ---------------------------------------------------------------------------

// Valid roll types.
var validRollTypes = map[string]struct{}{
	"attack": {},
	"damage": {},
	"check":  {},
	"save":   {},
}

// Maximum number of dice in a single roll request.
const maxDiceCount = 10
const maxFollowUpDepth = 1

// Maximum DC value (D&D 5e SRD max ~30, we allow a bit of headroom).
const maxDCValue = 50

// Minimum DC value (DC 0 means "no roll needed").
const minDCValue = 1

// dicePattern matches dice notation like "d20", "1d20", "2d6+3", "4d8-1".
var dicePattern = regexp.MustCompile(`^(?:(\d+)\s*)?[wd]\s*(\d+)(?:\s*[+-]\s*\d+)?$`)

var allowedDiceSides = map[int]struct{}{
	4:   {},
	6:   {},
	8:   {},
	10:  {},
	12:  {},
	20:  {},
	100: {},
}

// ValidateRollRequest checks a roll request against allowed types, dice
// notation, DC range, and the follow-up chain.  Returns a cleaned copy and
// a list of rejection reasons.  If the request is fully invalid (len(reasons)
// > 0 && reasons are fatal), the caller should drop it.
func ValidateRollRequest(req *RollRequest) (*RollRequest, []string) {
	return validateRollRequest(req, 0)
}

func validateRollRequest(req *RollRequest, depth int) (*RollRequest, []string) {
	if req == nil {
		return req, nil
	}

	var reasons []string

	// --- type ---
	if _, ok := validRollTypes[strings.ToLower(strings.TrimSpace(req.Type))]; !ok {
		reasons = append(reasons, fmt.Sprintf("invalid roll type %q", req.Type))
	}

	// --- dice ---
	if len(req.Dice) == 0 {
		reasons = append(reasons, "roll_request must contain at least one die")
	} else {
		totalDiceCount := 0
		for i, die := range req.Dice {
			count, ok := parseDiceNotation(die)
			if !ok {
				reasons = append(reasons, fmt.Sprintf("invalid dice notation %q at index %d", die, i))
				continue
			}
			totalDiceCount += count
			if totalDiceCount > maxDiceCount {
				reasons = append(reasons, fmt.Sprintf("too many dice requested (%d, max %d)", totalDiceCount, maxDiceCount))
				break
			}
		}
	}

	// --- DC ---
	if req.DC != nil {
		if *req.DC < minDCValue || *req.DC > maxDCValue {
			reasons = append(reasons, fmt.Sprintf("DC %d is outside allowed range [%d,%d]", *req.DC, minDCValue, maxDCValue))
		}
	}

	// --- label ---
	if strings.TrimSpace(req.Label) == "" {
		reasons = append(reasons, "roll_request label must not be empty")
	}

	// --- follow-up chain (prevent deep nesting) ---
	if req.FollowUpOnSuccess != nil {
		if depth >= maxFollowUpDepth {
			reasons = append(reasons, "follow_up_on_success exceeds maximum nesting depth")
		}
		if _, ok := validRollTypes[strings.ToLower(strings.TrimSpace(req.Type))]; !ok {
			reasons = append(reasons, "follow_up_on_success requires a valid roll type")
		} else if depth < maxFollowUpDepth {
			// Recursively validate follow-up.
			if _, followReasons := validateRollRequest(req.FollowUpOnSuccess, depth+1); len(followReasons) > 0 {
				for _, r := range followReasons {
					reasons = append(reasons, "follow_up_on_success: "+r)
				}
			}
		}
	}

	if len(reasons) > 0 {
		return nil, reasons
	}

	// Return a cleaned copy.
	cleaned := *req
	return &cleaned, nil
}

// isValidDiceNotation checks if a dice string matches the expected notation.
func isValidDiceNotation(die string) bool {
	_, ok := parseDiceNotation(die)
	return ok
}

func parseDiceNotation(die string) (int, bool) {
	d := strings.ToLower(strings.TrimSpace(die))
	matches := dicePattern.FindStringSubmatch(d)
	if len(matches) != 3 {
		return 0, false
	}
	count := 1
	if strings.TrimSpace(matches[1]) != "" {
		count = parseDiceCount(matches[1])
	}
	sides := parseDiceCount(matches[2])
	if count < 1 {
		return 0, false
	}
	if _, ok := allowedDiceSides[sides]; !ok {
		return 0, false
	}
	return count, true
}

func parseDiceCount(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ---------------------------------------------------------------------------
// Prompt untrusted-context delimiter
// ---------------------------------------------------------------------------

const (
	// Delimiter that wraps untrusted content (adventure text, uploaded docs).
	// The model must be told that anything inside these delimiters is untrusted
	// user content and must not be executed as instructions.
	untrustedContextDelimiter = "UNTRUSTED_CONTENT"
)

// wrapUntrustedContent wraps raw text in a delimiter that tells the LLM to
// treat it as untrusted user content.  This is a simple but effective
// prompt-injection mitigation: the system prompt explicitly says that
// instructions inside UNTRUSTED_CONTENT delimiters must not override system
// rules.
func wrapUntrustedContent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>", untrustedContextDelimiter, text, untrustedContextDelimiter)
}

// truncateUntrustedContent limits untrusted content to a safe maximum length
// to prevent prompt-injection via extremely long documents.
func truncateUntrustedContent(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "\n... [truncated]"
}
