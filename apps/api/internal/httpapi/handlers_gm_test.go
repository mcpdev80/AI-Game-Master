package httpapi

import "testing"

func TestCombatReadyCharactersFiltersJoinedAndReady(t *testing.T) {
	input := []map[string]any{
		{"id": "a", "status": "draft"},
		{"id": "b", "status": "joined"},
		{"id": "c", "status": "ready"},
	}
	got := combatReadyCharacters(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 ready combat characters, got %d", len(got))
	}
}

func TestInitializeCombatStateIncludesMultiplePlayers(t *testing.T) {
	activeCharacters := []map[string]any{
		{"id": "p1", "name": "Aria", "status": "ready", "abilities": map[string]int{"dexterity": 14}},
		{"id": "p2", "name": "Borin", "status": "joined", "abilities": map[string]int{"dexterity": 12}},
	}
	state := initializeCombatState(
		Session{},
		GMRespondRequest{
			PlayerInput: "Ich würfle Initiative gegen zwei Spinnen.",
			DiceRoll: &DiceRollEvent{
				Dice: []DiceResult{{Type: "d20", Value: 17}},
			},
		},
		GMResponse{Narration: "Zwei Spinnen stürzen aus der Dunkelheit."},
		activeCharacters,
	)
	if !state.Active {
		t.Fatal("expected combat to be active")
	}
	playerCount := 0
	for _, turn := range state.InitiativeOrder {
		if turn.Side == "player" {
			playerCount++
		}
	}
	if playerCount != 2 {
		t.Fatalf("expected 2 player turns in initiative order, got %d", playerCount)
	}
}

func TestEnsureInteractiveNarrationAddsGermanQuestion(t *testing.T) {
	got := ensureInteractiveNarration("de", "Die Tür schwingt knarrend auf.", nil)
	want := "Die Tür schwingt knarrend auf. Was tut ihr jetzt?"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnsureInteractiveNarrationKeepsExistingQuestion(t *testing.T) {
	got := ensureInteractiveNarration("en", "The torchlight flickers across the altar. What do you do?", nil)
	want := "The torchlight flickers across the altar. What do you do?"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnsureInteractiveNarrationSkipsRollPrompt(t *testing.T) {
	got := ensureInteractiveNarration("en", "Roll 1d20.", &RollRequest{Type: "check", Dice: []string{"1d20"}})
	want := "Roll 1d20."
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEmbeddedDocumentChunkUsesEmbeddedContent(t *testing.T) {
	document := Document{
		ID:   "embedded-short-rules-dnd-5e",
		Name: "short_rules",
		Metadata: map[string]any{
			"embedded_content": "Rule text",
		},
	}
	chunk, ok := embeddedDocumentChunk(document)
	if !ok {
		t.Fatal("expected embedded chunk")
	}
	if chunk.DocumentID != document.ID || chunk.DocumentName != document.Name || chunk.ChunkText != "Rule text" {
		t.Fatalf("unexpected chunk: %+v", chunk)
	}
}

func TestIsUUIDLike(t *testing.T) {
	if !isUUIDLike("ee9a659b-d887-4a07-a63c-48a1c8230b4a") {
		t.Fatal("expected UUID-like value to match")
	}
	if isUUIDLike("embedded-short-rules-dnd-5e") {
		t.Fatal("expected embedded id to be rejected")
	}
}

func TestParseCharacterLevel(t *testing.T) {
	if got := parseCharacterLevel("Fighter 5"); got != 5 {
		t.Fatalf("expected level 5, got %d", got)
	}
	if got := parseCharacterLevel("Kämpfer, Stufe 3"); got != 3 {
		t.Fatalf("expected level 3, got %d", got)
	}
	if got := parseCharacterLevel(""); got != 1 {
		t.Fatalf("expected fallback level 1, got %d", got)
	}
}

func TestCharacterEligibleForLevelUpByXPThreshold(t *testing.T) {
	character := Character{
		ClassAndLevel: "Fighter 1",
		Metadata: map[string]any{
			"experience_points": "300",
		},
	}
	if !characterEligibleForLevelUp(character) {
		t.Fatal("expected level-up eligibility at exact XP threshold")
	}
}

func TestCharacterEligibleForLevelUpByManualFlag(t *testing.T) {
	character := Character{
		ClassAndLevel: "Wizard 2",
		Metadata: map[string]any{
			"experience_points": 0,
			"level_up_available": "true",
		},
	}
	if !characterEligibleForLevelUp(character) {
		t.Fatal("expected manual level-up flag to be honored")
	}
}

func TestBuildLevelUpQueueIncludesEligibleCharactersOnly(t *testing.T) {
	queue := buildLevelUpQueue([]Character{
		{
			ID:            "c1",
			Name:          "Aria",
			ClassAndLevel: "Rogue 1",
			Metadata: map[string]any{
				"experience_points": "300",
			},
		},
		{
			ID:            "c2",
			Name:          "Borin",
			ClassAndLevel: "Cleric 1",
			Metadata: map[string]any{
				"experience_points": "120",
			},
		},
	}, "de")
	if len(queue) != 1 {
		t.Fatalf("expected 1 eligible character in queue, got %d", len(queue))
	}
	if queue[0].CharacterName != "Aria" || queue[0].CurrentLevel != 1 || queue[0].NextLevel != 2 {
		t.Fatalf("unexpected queue entry: %+v", queue[0])
	}
}

func TestBuildRewardSummary(t *testing.T) {
	summary := buildRewardSummary([]StateUpdate{
		{EntityID: "char-1", Field: "experience_points", Delta: 200},
		{EntityID: "session", Field: "group_gold", Delta: 15},
		{EntityID: "session", Field: "group_inventory_add", Value: "Spider Fang"},
	}, "en")
	if summary == "" {
		t.Fatal("expected non-empty reward summary")
	}
}

func TestDetectsCombatResolution(t *testing.T) {
	if !detectsCombatResolution("The last enemy falls and the combat is over.") {
		t.Fatal("expected combat resolution to be detected")
	}
	if detectsCombatResolution("I attack again.") {
		t.Fatal("did not expect combat resolution for ongoing attack")
	}
}

func TestDetectsRestTransition(t *testing.T) {
	if !detectsRestTransition("We take a long rest at camp.") {
		t.Fatal("expected rest transition to be detected")
	}
	if detectsRestTransition("We charge into battle.") {
		t.Fatal("did not expect rest transition in combat action")
	}
}

func TestShouldAutoStartCombatForHostileAttackRoll(t *testing.T) {
	session := Session{}
	req := GMRespondRequest{
		PlayerInput: "I draw my crossbow and shoot the spider.",
	}
	response := GMResponse{
		Narration: "Two giant spiders descend from the ceiling.",
		RollRequest: &RollRequest{
			Type:  "attack",
			Label: "Light Crossbow Attack",
			Dice:  []string{"1d20+4"},
		},
	}
	if !shouldAutoStartCombat(session, req, response) {
		t.Fatal("expected hostile attack setup to auto-start combat")
	}
}

func TestShouldNotAutoStartCombatWithoutEnemies(t *testing.T) {
	session := Session{}
	req := GMRespondRequest{
		PlayerInput: "I inspect the old altar.",
	}
	response := GMResponse{
		Narration: "The chamber is quiet and filled with dust.",
		RollRequest: &RollRequest{
			Type:  "check",
			Label: "Religion Check",
			Dice:  []string{"1d20+2"},
		},
	}
	if shouldAutoStartCombat(session, req, response) {
		t.Fatal("did not expect peaceful scene to auto-start combat")
	}
}

func TestDetectCombatEnemyNamesBalancesSoloLevelOneSpiderEncounter(t *testing.T) {
	activeCharacters := []map[string]any{
		{"id": "p1", "name": "Thoras", "status": "ready", "class_and_level": "Fighter 1"},
	}
	got := detectCombatEnemyNames(Session{}, "I attack the spider.", GMResponse{Narration: "Two giant spiders drop from the ceiling."}, activeCharacters)
	if len(got) != 1 || got[0] != "Hunting Spider" {
		t.Fatalf("expected one downgraded solo spider encounter, got %#v", got)
	}
}

func TestDetectCombatEnemyNamesKeepsPluralButDowngradesLowLevelDuos(t *testing.T) {
	activeCharacters := []map[string]any{
		{"id": "p1", "name": "Thoras", "status": "ready", "class_and_level": "Fighter 1"},
		{"id": "p2", "name": "Mira", "status": "joined", "class_and_level": "Cleric 1"},
	}
	got := detectCombatEnemyNames(Session{}, "We face spiders.", GMResponse{Narration: "Two giant spiders crawl out of the webbing."}, activeCharacters)
	if len(got) != 2 || got[0] != "Hunting Spider 1" || got[1] != "Hunting Spider 2" {
		t.Fatalf("expected two downgraded spiders for level-one duo, got %#v", got)
	}
}

func TestEnemyProfileForNameUsesDowngradedSpiderStats(t *testing.T) {
	got := enemyProfileForName("Hunting Spider")
	if got.AttackBonus != 3 || got.DamageDice != "1d4+1" {
		t.Fatalf("expected downgraded hunting spider profile, got %+v", got)
	}
}

func TestDetectCombatEnemyNamesBalancesLowLevelGoblins(t *testing.T) {
	activeCharacters := []map[string]any{
		{"id": "p1", "name": "Thoras", "status": "ready", "class_and_level": "Fighter 1"},
		{"id": "p2", "name": "Mira", "status": "joined", "class_and_level": "Wizard 1"},
	}
	got := detectCombatEnemyNames(Session{}, "We attack the goblins.", GMResponse{Narration: "Two goblins rush from cover."}, activeCharacters)
	if len(got) != 2 || got[0] != "Goblin Scout 1" || got[1] != "Goblin Scout 2" {
		t.Fatalf("expected two goblin scouts for low-level duo, got %#v", got)
	}
}

func TestDetectCombatEnemyNamesBalancesHighLevelWolves(t *testing.T) {
	activeCharacters := []map[string]any{
		{"id": "p1", "name": "Thoras", "status": "ready", "class_and_level": "Fighter 4"},
		{"id": "p2", "name": "Mira", "status": "joined", "class_and_level": "Cleric 4"},
		{"id": "p3", "name": "Eryn", "status": "joined", "class_and_level": "Ranger 4"},
		{"id": "p4", "name": "Lysa", "status": "joined", "class_and_level": "Wizard 4"},
	}
	got := detectCombatEnemyNames(Session{}, "The wolves circle us.", GMResponse{Narration: "A wolf pack closes in."}, activeCharacters)
	if len(got) != 2 || got[0] != "Dire Wolf 1" || got[1] != "Dire Wolf 2" {
		t.Fatalf("expected two dire wolves for stronger party, got %#v", got)
	}
}

func TestEnemyProfileForNameUsesGoblinVariantStats(t *testing.T) {
	got := enemyProfileForName("Goblin Scout")
	if got.AttackBonus != 3 || got.DamageDice != "1d4+1" || got.DamageType != "slashing" {
		t.Fatalf("expected goblin scout profile, got %+v", got)
	}
}
