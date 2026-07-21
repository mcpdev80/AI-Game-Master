type TrFn = (en: string, de: string) => string;

type CombatTurnLike = {
  id: string;
  character_id?: string;
  name: string;
  side: string;
  participant_type?: string;
  status?: string;
  hit_point_max?: number;
  current_hit_points?: number;
  temporary_hit_points?: number;
  death_save_successes?: number;
  death_save_failures?: number;
  stable?: boolean;
};

type CharacterLike = {
  id: string;
  name: string;
  hit_point_max: number | null;
  metadata: Record<string, unknown>;
};

export type CombatIndicator = {
  level: "healthy" | "wounded" | "critical" | "near-death" | "downed" | "stable" | "dead" | "unknown";
  marker: string;
  label: string;
  details: string;
};

function parseNumeric(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim()) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

function parseBoolean(value: unknown): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return ["true", "1", "yes", "ja", "stable", "stabil"].includes(normalized);
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  return false;
}

function currentHitPoints(character: CharacterLike): number | null {
  const current = parseNumeric(character.metadata.current_hit_points);
  if (current !== null) {
    return current;
  }
  return character.hit_point_max;
}

function turnCurrentHitPoints(turn: CombatTurnLike): number | null {
  return parseNumeric(turn.current_hit_points);
}

function turnTemporaryHitPoints(turn: CombatTurnLike): number {
  return Math.max(0, parseNumeric(turn.temporary_hit_points) ?? 0);
}

function turnDeathSaveSuccesses(turn: CombatTurnLike): number {
  return Math.max(0, Math.min(3, parseNumeric(turn.death_save_successes) ?? 0));
}

function turnDeathSaveFailures(turn: CombatTurnLike): number {
  return Math.max(0, Math.min(3, parseNumeric(turn.death_save_failures) ?? 0));
}

function turnStable(turn: CombatTurnLike): boolean {
  return parseBoolean(turn.stable);
}

function temporaryHitPoints(character: CharacterLike): number {
  return Math.max(0, parseNumeric(character.metadata.temporary_hit_points) ?? 0);
}

function deathSaveSuccesses(character: CharacterLike): number {
  return Math.max(0, Math.min(3, parseNumeric(character.metadata.death_save_successes) ?? 0));
}

function deathSaveFailures(character: CharacterLike): number {
  return Math.max(0, Math.min(3, parseNumeric(character.metadata.death_save_failures) ?? 0));
}

function deathSaveStable(character: CharacterLike): boolean {
  return (
    parseBoolean(character.metadata.death_save_stable) ||
    parseBoolean(character.metadata.stable) ||
    parseBoolean(character.metadata.is_stable)
  );
}

function hpLevelForRatio(ratio: number): CombatIndicator["level"] {
  if (ratio > 0.75) return "healthy";
  if (ratio > 0.5) return "wounded";
  if (ratio > 0.25) return "critical";
  if (ratio > 0.05) return "near-death";
  return "downed";
}

function markerForLevel(level: CombatIndicator["level"]): string {
  switch (level) {
    case "dead":
      return "☠";
    case "stable":
      return "◌";
    case "downed":
      return "●";
    default:
      return "●";
  }
}

export function findCombatCharacter(
  turn: CombatTurnLike,
  characters: CharacterLike[],
  playerLinks: Array<{ player_slot: { character_id: string | null } }>
): CharacterLike | null {
  const byID = characters.find((character) => character.id === turn.id);
  if (byID) {
    return byID;
  }
  if (turn.character_id) {
    const byCharacterID = characters.find((character) => character.id === turn.character_id);
    if (byCharacterID) {
      return byCharacterID;
    }
  }
  if (turn.side !== "player") {
    return null;
  }
  const linkedIDs = new Set(playerLinks.map((slot) => slot.player_slot.character_id).filter(Boolean));
  return characters.find((character) => linkedIDs.has(character.id)) ?? null;
}

export function combatIndicatorForTurn(turn: CombatTurnLike, character: CharacterLike | null, tr: TrFn): CombatIndicator {
  const turnMaxHP = Math.max(0, parseNumeric(turn.hit_point_max) ?? 0);
  const turnCurrentHP = turnCurrentHitPoints(turn);
  const turnTempHP = turnTemporaryHitPoints(turn);
  const turnSuccesses = turnDeathSaveSuccesses(turn);
  const turnFailures = turnDeathSaveFailures(turn);
  const turnIsStable = turnStable(turn);
  if (turnCurrentHP !== null && turnMaxHP >= 0) {
    if (turnCurrentHP <= 0) {
      if (turnFailures >= 3 || String(turn.status ?? "").toLowerCase().includes("dead")) {
        return {
          level: "dead",
          marker: markerForLevel("dead"),
          label: tr("Dead", "Tot"),
          details: tr("3 failed death saves", "3 misslungene Todessaves"),
        };
      }
      if (turnIsStable || turnSuccesses >= 3) {
        return {
          level: "stable",
          marker: markerForLevel("stable"),
          label: tr("Stable", "Stabil"),
          details: tr(`0 HP · ${turnSuccesses} successes / ${turnFailures} failures`, `0 TP · ${turnSuccesses} Erfolge / ${turnFailures} Fehlschläge`),
        };
      }
      return {
        level: "downed",
        marker: markerForLevel("downed"),
        label: tr("Dying", "Am Boden"),
        details: tr(`0 HP · ${turnSuccesses} successes / ${turnFailures} failures`, `0 TP · ${turnSuccesses} Erfolge / ${turnFailures} Fehlschläge`),
      };
    }
    if (turnMaxHP > 0) {
      const ratio = Math.max(0, turnCurrentHP) / turnMaxHP;
      const level = hpLevelForRatio(ratio);
      const baseDetails = turnTempHP > 0 ? `${turnCurrentHP}/${turnMaxHP} HP + ${turnTempHP} THP` : `${turnCurrentHP}/${turnMaxHP} HP`;
      return {
        level,
        marker: markerForLevel(level),
        label:
          level === "healthy"
            ? tr("Healthy", "Fit")
            : level === "wounded"
            ? tr("Wounded", "Angeschlagen")
            : level === "critical"
            ? tr("Critical", "Kritisch")
            : level === "near-death"
            ? tr("Near Death", "Fast tot")
            : tr("Down", "Am Boden"),
        details: baseDetails,
      };
    }
  }
  if (character) {
    const maxHP = Math.max(0, character.hit_point_max ?? 0);
    const currentHP = currentHitPoints(character);
    const tempHP = temporaryHitPoints(character);
    const successes = deathSaveSuccesses(character);
    const failures = deathSaveFailures(character);
    const stable = deathSaveStable(character);

    if (currentHP !== null && currentHP <= 0) {
      if (failures >= 3) {
        return {
          level: "dead",
          marker: markerForLevel("dead"),
          label: tr("Dead", "Tot"),
          details: tr("3 failed death saves", "3 misslungene Todessaves"),
        };
      }
      if (stable || successes >= 3) {
        return {
          level: "stable",
          marker: markerForLevel("stable"),
          label: tr("Stable", "Stabil"),
          details: tr(`0 HP · ${successes} successes / ${failures} failures`, `0 TP · ${successes} Erfolge / ${failures} Fehlschläge`),
        };
      }
      return {
        level: "downed",
        marker: markerForLevel("downed"),
        label: tr("Dying", "Am Boden"),
        details: tr(`0 HP · ${successes} successes / ${failures} failures`, `0 TP · ${successes} Erfolge / ${failures} Fehlschläge`),
      };
    }

    if (currentHP !== null && maxHP > 0) {
      const ratio = Math.max(0, currentHP) / maxHP;
      const level = hpLevelForRatio(ratio);
      const baseDetails = tempHP > 0 ? `${currentHP}/${maxHP} HP + ${tempHP} THP` : `${currentHP}/${maxHP} HP`;
      return {
        level,
        marker: markerForLevel(level),
        label:
          level === "healthy"
            ? tr("Healthy", "Fit")
            : level === "wounded"
            ? tr("Wounded", "Angeschlagen")
            : level === "critical"
            ? tr("Critical", "Kritisch")
            : level === "near-death"
            ? tr("Near Death", "Fast tot")
            : tr("Down", "Am Boden"),
        details: baseDetails,
      };
    }
  }

  const normalizedStatus = String(turn.status ?? "").trim().toLowerCase();
  if (normalizedStatus.includes("dead") || normalizedStatus.includes("slain") || normalizedStatus.includes("killed") || normalizedStatus.includes("tot")) {
    return { level: "dead", marker: markerForLevel("dead"), label: tr("Dead", "Tot"), details: tr("Defeated", "Besiegt") };
  }
  if (normalizedStatus.includes("stable") || normalizedStatus.includes("stabil")) {
    return { level: "stable", marker: markerForLevel("stable"), label: tr("Stable", "Stabil"), details: tr("No immediate danger", "Keine akute Gefahr") };
  }
  if (normalizedStatus.includes("down") || normalizedStatus.includes("unconscious") || normalizedStatus.includes("am boden") || normalizedStatus.includes("bewusstlos")) {
    return { level: "downed", marker: markerForLevel("downed"), label: tr("Down", "Am Boden"), details: tr("Unable to act", "Handlungsunfähig") };
  }
  if (normalizedStatus.includes("critical") || normalizedStatus.includes("near death")) {
    return { level: "near-death", marker: markerForLevel("near-death"), label: tr("Near Death", "Fast tot"), details: tr("Barely holding on", "Hält sich kaum noch") };
  }
  if (normalizedStatus.includes("bloodied") || normalizedStatus.includes("wounded") || normalizedStatus.includes("injured")) {
    return { level: "critical", marker: markerForLevel("critical"), label: tr("Wounded", "Verwundet"), details: tr("Clearly hurt", "Sichtbar verletzt") };
  }
  if (turn.side === "enemy") {
    return { level: "unknown", marker: "●", label: tr("Unknown", "Unbekannt"), details: tr("No exact HP tracked yet", "Noch keine exakten TP erfasst") };
  }
  return { level: "unknown", marker: "●", label: tr("Unknown", "Unbekannt"), details: tr("No combat vitals available", "Keine Kampfdaten verfügbar") };
}
