package httpapi

import "testing"

func TestLocalizedMeasurementTextGermanConvertsFeetToMeters(t *testing.T) {
	got := localizedMeasurementText("120 feet", "de")
	if got != "36 m" {
		t.Fatalf("expected 120 feet to become 36 m, got %q", got)
	}
}

func TestLocalizedMeasurementTextGermanConvertsFootCubeToMeters(t *testing.T) {
	got := localizedMeasurementText("Self (15-foot cube)", "de")
	if got != "Self (4,5 m-Würfel)" {
		t.Fatalf("expected foot cube localization, got %q", got)
	}
}

func TestLocalizedMeasurementTextEnglishConvertsGermanFeetToFt(t *testing.T) {
	got := localizedMeasurementText("30 Fuß", "en")
	if got != "30 ft" {
		t.Fatalf("expected 30 Fuß to become 30 ft, got %q", got)
	}
}

func TestBuilderDerivedSpeedLocalizesByCharacterLanguage(t *testing.T) {
	de := builderDerivedSpeed(Character{
		Race: "Mensch",
		Metadata: map[string]any{
			"language": "de",
		},
	})
	if de != "9 m" {
		t.Fatalf("expected german speed in meters, got %q", de)
	}

	en := builderDerivedSpeed(Character{
		Race: "Mensch",
		Metadata: map[string]any{
			"language": "en",
		},
	})
	if en != "30 ft" {
		t.Fatalf("expected english speed in feet, got %q", en)
	}
}
