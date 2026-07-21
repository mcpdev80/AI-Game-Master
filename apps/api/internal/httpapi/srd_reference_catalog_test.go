package httpapi

import "testing"

func TestSRDMonsterCatalogContainsExpectedCount(t *testing.T) {
	if got := len(srdMonsterCatalog); got != 319 {
		t.Fatalf("expected 319 SRD monsters, got %d", got)
	}
}

func TestSRDMonsterCatalogLookupGoblin(t *testing.T) {
	entry, ok := srdMonsterCatalogEntryByName("Goblin")
	if !ok {
		t.Fatal("expected goblin in SRD monster catalog")
	}
	if entry.Name != "Goblin" {
		t.Fatalf("expected Goblin entry, got %q", entry.Name)
	}
	if entry.AttackBonus != 4 {
		t.Fatalf("expected goblin attack bonus 4, got %d", entry.AttackBonus)
	}
	if entry.DamageDice != "1d6+2" {
		t.Fatalf("expected goblin damage dice 1d6+2, got %q", entry.DamageDice)
	}
	if entry.DamageType != "slashing" {
		t.Fatalf("expected goblin damage type slashing, got %q", entry.DamageType)
	}
	if entry.SourcePage != 315 {
		t.Fatalf("expected goblin source page 315, got %d", entry.SourcePage)
	}
}

func TestSRDMonsterCatalogLookupWolf(t *testing.T) {
	entry, ok := srdMonsterCatalogEntryByName("Wolf")
	if !ok {
		t.Fatal("expected wolf in SRD monster catalog")
	}
	if entry.AttackBonus != 4 || entry.DamageDice != "2d4+2" || entry.DamageType != "piercing" {
		t.Fatalf("unexpected wolf entry: %+v", entry)
	}
}

func TestSRDMonsterCatalogLookupGiantSpider(t *testing.T) {
	entry, ok := srdMonsterCatalogEntryByName("Giant Spider")
	if !ok {
		t.Fatal("expected giant spider in SRD monster catalog")
	}
	if entry.AttackBonus != 5 || entry.DamageDice != "1d8+3" || entry.DamageType != "piercing" {
		t.Fatalf("unexpected giant spider entry: %+v", entry)
	}
}
