"use client";

import { useEffect, useMemo, useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Bot, Check, Copy, MessageSquare, Mic, MicOff, PenSquare, Plus, Trash2, User, Volume2 } from "lucide-react";
import { PageIntro, Panel, StatCard, StatusPill } from "../studio-primitives";
import { useNotifications } from "../notifications-provider";
import { useI18n } from "../../lib/i18n";
import {
  apiDelete,
  apiUploadRaw,
  applyCharacterBuilderPatch,
  createCharacter,
  detectDiceFromImage,
  finishCharacterBuilder,
  resolveAbilityScores,
  sendCharacterBuilderMessage,
  splitMetadataList,
  startCharacterBuilder,
  updatePlayerSlotCharacter,
  validateAbilityAssignment,
  apiBaseUrl,
  type Adventure,
  type Campaign,
  type Character,
  type CharacterBuilderMessage,
  type CharacterBuilderPatch,
  type Document,
} from "../../lib/api";

type CharactersScreenProps = {
  characters: Character[];
  campaigns: Campaign[];
  adventures: Adventure[];
  documents: Document[];
  initialBuilderSeed?: {
    portalToken: string;
    returnPath: string;
    playerSlotId: string;
    characterName?: string;
    playerName: string;
    campaignId: string;
    rulesetWork: string;
    rulesetVersion: string;
  } | null;
};

type AbilityKey = "strength" | "dexterity" | "constitution" | "intelligence" | "wisdom" | "charisma";
type BuilderStep = "start" | "chat";
type AbilityMethod = "standard" | "point_buy" | "rolled";
type SheetTab = "overview" | "abilities" | "combat" | "magic" | "personality" | "gear";
const STT_TARGET_SAMPLE_RATE = 32000;
const BUILDER_MAX_RECORDING_MS = 10000;

type SheetFormState = {
  name: string;
  player_name: string;
  class_and_level: string;
  race: string;
  background: string;
  alignment: string;
  languages: string;
  armor_class: string;
  hit_point_max: string;
  current_hit_points: string;
  temporary_hit_points: string;
  speed: string;
  proficiency_bonus: string;
  inspiration: string;
  experience_points: string;
  hit_dice: string;
  concept: string;
  builder_stage: string;
  personality_traits: string;
  ideals: string;
  bonds: string;
  flaws: string;
  backstory: string;
  age: string;
  size: string;
  weight: string;
  eyes: string;
  skin: string;
  hair: string;
  allies: string;
  senses: string;
  tools_and_proficiencies: string;
  weapon_notes: string;
  starting_money: string;
  current_money: string;
  current_inventory: string;
  level_up_available: string;
  combat_overview: string;
  combat_attacks: string;
  skill_proficiencies: string;
  saving_throw_proficiencies: string;
  starting_equipment: string;
  spells: string;
  spell_save_dc: string;
  spell_attack_bonus: string;
  spell_attacks: string;
  spell_notes: string;
};
const guidedRollCount = 7;
const keptRollCount = 6;

type RulesetGroup = {
  key: string;
  work: string;
  version: string;
  documents: Document[];
};

type RosterCharacter = Character & {
  rulesetLabel: string;
  statusLabel: string;
  concept: string;
  selectedDocumentNames: string[];
};

type ParsedTableRow = {
  columns: string[];
  description: string;
};

const abilityOrder: AbilityKey[] = ["strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"];
const skillDefinitions: Array<{ key: string; label: string; ability: AbilityKey }> = [
  { key: "acrobatics", label: "Akrobatik", ability: "dexterity" },
  { key: "arcana", label: "Arkane Kunde", ability: "intelligence" },
  { key: "athletics", label: "Athletik", ability: "strength" },
  { key: "performance", label: "Auftreten", ability: "charisma" },
  { key: "intimidation", label: "Einschüchtern", ability: "charisma" },
  { key: "sleight_of_hand", label: "Fingerfertigkeit", ability: "dexterity" },
  { key: "history", label: "Geschichte", ability: "intelligence" },
  { key: "medicine", label: "Heilkunde", ability: "wisdom" },
  { key: "stealth", label: "Heimlichkeit", ability: "dexterity" },
  { key: "animal_handling", label: "Mit Tieren umgehen", ability: "wisdom" },
  { key: "insight", label: "Motiv erkennen", ability: "wisdom" },
  { key: "investigation", label: "Nachforschungen", ability: "intelligence" },
  { key: "nature", label: "Naturkunde", ability: "intelligence" },
  { key: "religion", label: "Religion", ability: "intelligence" },
  { key: "deception", label: "Täuschen", ability: "charisma" },
  { key: "survival", label: "Überlebenskunst", ability: "wisdom" },
  { key: "persuasion", label: "Überzeugen", ability: "charisma" },
  { key: "perception", label: "Wahrnehmung", ability: "wisdom" },
];
const skillAliases: Record<string, string> = {
  akrobatik: "acrobatics",
  "arkane kunde": "arcana",
  athletik: "athletics",
  auftreten: "performance",
  "einschüchtern": "intimidation",
  einschuchtern: "intimidation",
  fingerfertigkeit: "sleight_of_hand",
  geschichte: "history",
  heilkunde: "medicine",
  heimlichkeit: "stealth",
  "mit tieren umgehen": "animal_handling",
  "motiv erkennen": "insight",
  nachforschungen: "investigation",
  naturkunde: "nature",
  religion: "religion",
  täuschen: "deception",
  taeuschen: "deception",
  "überlebenskunst": "survival",
  ueberlebenskunst: "survival",
  überleben: "survival",
  ueberleben: "survival",
  überzeugen: "persuasion",
  ueberzeugen: "persuasion",
  wahrnehmung: "perception",
};
const abilityLabels: Record<AbilityKey, string> = {
  strength: "STÄRKE",
  dexterity: "GESCHICKLICHKEIT",
  constitution: "KONSTITUTION",
  intelligence: "INTELLIGENZ",
  wisdom: "WEISHEIT",
  charisma: "CHARISMA",
};
const englishAbilityLabels: Record<AbilityKey, string> = {
  strength: "STRENGTH",
  dexterity: "DEXTERITY",
  constitution: "CONSTITUTION",
  intelligence: "INTELLIGENCE",
  wisdom: "WISDOM",
  charisma: "CHARISMA",
};
const englishSkillLabels: Record<string, string> = {
  acrobatics: "Acrobatics", arcana: "Arcana", athletics: "Athletics", performance: "Performance",
  intimidation: "Intimidation", sleight_of_hand: "Sleight of Hand", history: "History", medicine: "Medicine",
  stealth: "Stealth", animal_handling: "Animal Handling", insight: "Insight", investigation: "Investigation",
  nature: "Nature", religion: "Religion", deception: "Deception", survival: "Survival",
  persuasion: "Persuasion", perception: "Perception",
};

function deriveRuleset(metadata: Record<string, unknown>) {
  const work = String(metadata.ruleset_work ?? "");
  const version = String(metadata.ruleset_version ?? "");
  if (work || version) {
    return { work: work || "Unassigned", version: version || "default" };
  }
  const fallback = splitMetadataList(metadata.ruleset_keys)[0] ?? "";
  if (fallback.includes(":")) {
    const [fallbackWork, fallbackVersion] = fallback.split(":");
    return { work: fallbackWork || "Unassigned", version: fallbackVersion || "default" };
  }
  return { work: "Unassigned", version: "default" };
}

function normalizeSkillToken(value: string) {
  const token = value.trim().toLowerCase();
  return skillAliases[token] ?? token;
}

function createEmptyAssignment() {
  return {
    strength: 0,
    dexterity: 0,
    constitution: 0,
    intelligence: 0,
    wisdom: 0,
    charisma: 0,
  };
}

function normalizeAssignment(values: Record<string, number>): Record<AbilityKey, number> {
  return {
    strength: values.strength ?? 0,
    dexterity: values.dexterity ?? 0,
    constitution: values.constitution ?? 0,
    intelligence: values.intelligence ?? 0,
    wisdom: values.wisdom ?? 0,
    charisma: values.charisma ?? 0,
  };
}

function normalizeSuggestedAssignment(value: unknown): Record<AbilityKey, number> {
  if (!value || typeof value !== "object") {
    return createEmptyAssignment();
  }
  const raw = value as Record<string, unknown>;
  return {
    strength: Number(raw.strength) || 0,
    dexterity: Number(raw.dexterity) || 0,
    constitution: Number(raw.constitution) || 0,
    intelligence: Number(raw.intelligence) || 0,
    wisdom: Number(raw.wisdom) || 0,
    charisma: Number(raw.charisma) || 0,
  };
}

function parseBuilderMessages(metadata: Record<string, unknown>): CharacterBuilderMessage[] {
  const raw = metadata.builder_messages;
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw
    .map((entry) => {
      if (!entry || typeof entry !== "object") {
        return null;
      }
      const item = entry as Record<string, unknown>;
      const role = String(item.role ?? "").trim();
      const content = String(item.content ?? "").trim();
      const createdAt = String(item.created_at ?? new Date().toISOString());
      if (!role || !content) {
        return null;
      }
      return { role, content, created_at: createdAt };
    })
    .filter((entry): entry is CharacterBuilderMessage => Boolean(entry));
}

function parseRollSets(text: string): number[][] {
  return text
    .split("\n")
    .map((line) =>
      line
        .split(/[,\s]+/)
        .map((chunk) => Number(chunk.trim()))
        .filter((value) => Number.isFinite(value))
    )
    .filter((line) => line.length > 0);
}

function buildBuilderSpeechText(value: string) {
  return value.replace(/\s+/g, " ").trim();
}

type AudioContextConstructor = typeof AudioContext;

function getAudioContextConstructor(): AudioContextConstructor | null {
  return (
    window.AudioContext ||
    (window as typeof window & { webkitAudioContext?: AudioContextConstructor }).webkitAudioContext ||
    null
  );
}

function pickRecordingMimeType(): string {
  const candidates = ["audio/mp4", "audio/ogg;codecs=opus", "audio/webm;codecs=opus"];
  for (const candidate of candidates) {
    if (typeof MediaRecorder !== "undefined" && MediaRecorder.isTypeSupported(candidate)) {
      return candidate;
    }
  }
  return "";
}

function writeAscii(view: DataView, offset: number, value: string) {
  for (let index = 0; index < value.length; index += 1) {
    view.setUint8(offset + index, value.charCodeAt(index));
  }
}

function mixToMono(audioBuffer: AudioBuffer): Float32Array {
  const channels = Math.max(1, audioBuffer.numberOfChannels);
  if (channels === 1) {
    return audioBuffer.getChannelData(0).slice();
  }
  const output = new Float32Array(audioBuffer.length);
  for (let channelIndex = 0; channelIndex < channels; channelIndex += 1) {
    const data = audioBuffer.getChannelData(channelIndex);
    for (let sampleIndex = 0; sampleIndex < data.length; sampleIndex += 1) {
      output[sampleIndex] += data[sampleIndex] / channels;
    }
  }
  return output;
}

function resampleMonoBuffer(input: Float32Array, sourceSampleRate: number, targetSampleRate: number): Float32Array {
  if (!Number.isFinite(sourceSampleRate) || sourceSampleRate <= 0 || sourceSampleRate === targetSampleRate) {
    return input;
  }
  const targetLength = Math.max(1, Math.round(input.length * targetSampleRate / sourceSampleRate));
  const output = new Float32Array(targetLength);
  const ratio = sourceSampleRate / targetSampleRate;
  for (let index = 0; index < targetLength; index += 1) {
    const sourceIndex = index * ratio;
    const leftIndex = Math.floor(sourceIndex);
    const rightIndex = Math.min(leftIndex + 1, input.length - 1);
    const weight = sourceIndex - leftIndex;
    const left = input[leftIndex] ?? 0;
    const right = input[rightIndex] ?? left;
    output[index] = left + (right - left) * weight;
  }
  return output;
}

function audioBufferToWavBlob(audioBuffer: AudioBuffer): Blob {
  const sampleRate = STT_TARGET_SAMPLE_RATE;
  const channelCount = 1;
  const channelData = resampleMonoBuffer(mixToMono(audioBuffer), audioBuffer.sampleRate, sampleRate);
  const samples = channelData.length;
  const bytesPerSample = 2;
  const blockAlign = channelCount * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = samples * blockAlign;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);

  writeAscii(view, 0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeAscii(view, 8, "WAVE");
  writeAscii(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, channelCount, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, 16, true);
  writeAscii(view, 36, "data");
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (let index = 0; index < channelData.length; index += 1) {
    const sample = Math.max(-1, Math.min(1, channelData[index] ?? 0));
    const pcmValue = sample < 0 ? sample * 0x8000 : sample * 0x7fff;
    view.setInt16(offset, pcmValue, true);
    offset += 2;
  }

  return new Blob([buffer], { type: "audio/wav" });
}

async function blobToWav(blob: Blob): Promise<Blob> {
  const AudioContextCtor = getAudioContextConstructor();
  if (!AudioContextCtor) {
    throw new Error("AudioContext wird von diesem Browser nicht unterstützt.");
  }
  const arrayBuffer = await blob.arrayBuffer();
  const audioContext = new AudioContextCtor();
  try {
    const decoded = await audioContext.decodeAudioData(arrayBuffer.slice(0));
    return audioBufferToWavBlob(decoded);
  } finally {
    await audioContext.close().catch(() => undefined);
  }
}

async function uploadSTTBlob(blob: Blob, fallbackFilename: string, language?: string): Promise<string> {
  const formData = new FormData();
  try {
    const wavBlob = await blobToWav(blob);
    formData.append("file", wavBlob, fallbackFilename.replace(/\.[^.]+$/, ".wav"));
  } catch {
    formData.append("file", blob, fallbackFilename);
  }
  if (language) {
    formData.append("language", language);
  }
  const result = await apiUploadRaw<{ text: string }>("/api/stt/transcriptions", formData);
  return String(result.text || "").trim();
}

function speakWithBrowserVoice(text: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (typeof window === "undefined" || !("speechSynthesis" in window) || typeof SpeechSynthesisUtterance === "undefined") {
      reject(new Error("Sprachausgabe konnte nicht abgespielt werden."));
      return;
    }
    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = "de-DE";
    utterance.rate = 1;
    utterance.pitch = 1;
    utterance.onend = () => resolve();
    utterance.onerror = () => reject(new Error("Sprachausgabe konnte nicht abgespielt werden."));
    window.speechSynthesis.cancel();
    window.speechSynthesis.speak(utterance);
  });
}

function safeString(value: unknown) {
  return typeof value === "string" ? value : "";
}

function metadataStructuredText(value: unknown) {
  if (typeof value === "string") {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean).join("\n");
  }
  return "";
}

function metadataListToText(value: unknown) {
  return splitMetadataList(value).join(", ");
}

function abilityModifier(score: number) {
  return Math.floor((score - 10) / 2);
}

function formatModifier(value: number) {
  return value >= 0 ? `+${value}` : `${value}`;
}

function parseNumericText(value: string) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function hasResolvedRollData(metadata: Record<string, unknown>) {
  const resolvedValues = metadata.resolved_values;
  const rolledSets = metadata.rolled_sets;
  const hasResolvedValues = Array.isArray(resolvedValues) && resolvedValues.length >= keptRollCount;
  const hasRolledSets = Array.isArray(rolledSets) && rolledSets.length >= guidedRollCount;
  return hasResolvedValues || hasRolledSets;
}

function ensureRolledSets(text: string): number[][] {
  const parsed = parseRollSets(text)
    .slice(0, guidedRollCount)
    .map((set) => {
      const next = [...set].slice(0, 4);
      while (next.length < 4) {
        next.push(0);
      }
      return next;
    });
  while (parsed.length < guidedRollCount) {
    parsed.push([0, 0, 0, 0]);
  }
  return parsed;
}

function parseCharacterLevel(classAndLevel: string) {
  const match = classAndLevel.match(/level\s*(\d+)|stufe\s*(\d+)|\b(\d+)\b/i);
  const raw = match?.[1] ?? match?.[2] ?? match?.[3];
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function normalizeClassName(classAndLevel: string) {
  const value = classAndLevel.toLowerCase();
  if (value.includes("barbar")) return "barbarian";
  if (value.includes("bard")) return "bard";
  if (value.includes("cleric") || value.includes("kler")) return "cleric";
  if (value.includes("druid") || value.includes("druide")) return "druid";
  if (value.includes("fighter") || value.includes("kämpfer") || value.includes("kaempfer")) return "fighter";
  if (value.includes("monk") || value.includes("mönch") || value.includes("moench")) return "monk";
  if (value.includes("paladin")) return "paladin";
  if (value.includes("ranger") || value.includes("waldläufer") || value.includes("waldlaeufer")) return "ranger";
  if (value.includes("rogue") || value.includes("schurke")) return "rogue";
  if (value.includes("sorcerer") || value.includes("zauberer")) return "sorcerer";
  if (value.includes("warlock") || value.includes("hexenmeister")) return "warlock";
  if (value.includes("wizard") || value.includes("magier")) return "wizard";
  return "";
}

function deriveProficiencyBonus(classAndLevel: string) {
  const level = parseCharacterLevel(classAndLevel);
  return Math.floor((level - 1) / 4) + 2;
}

function deriveHitDie(classAndLevel: string) {
  switch (normalizeClassName(classAndLevel)) {
    case "barbarian":
      return "1d12";
    case "fighter":
    case "paladin":
    case "ranger":
      return "1d10";
    case "bard":
    case "cleric":
    case "druid":
    case "monk":
    case "rogue":
    case "warlock":
      return "1d8";
    case "sorcerer":
    case "wizard":
      return "1d6";
    default:
      return "";
  }
}

function deriveSavingThrowProficiencies(classAndLevel: string) {
  switch (normalizeClassName(classAndLevel)) {
    case "barbarian":
      return "Stärke, Konstitution";
    case "bard":
      return "Geschicklichkeit, Charisma";
    case "cleric":
    case "druid":
    case "warlock":
    case "wizard":
      return "Weisheit, Charisma";
    case "fighter":
      return "Stärke, Konstitution";
    case "monk":
      return "Stärke, Geschicklichkeit";
    case "paladin":
      return "Weisheit, Charisma";
    case "ranger":
      return "Stärke, Geschicklichkeit";
    case "rogue":
      return "Geschicklichkeit, Intelligenz";
    case "sorcerer":
      return "Konstitution, Charisma";
    default:
      return "";
  }
}

function deriveBaseSpeed(race: string) {
  const value = race.toLowerCase();
  if (value.includes("zwerg") || value.includes("dwarf") || value.includes("gnom") || value.includes("gnome") || value.includes("halbling") || value.includes("halfling")) {
    return "25 ft";
  }
  if (
    value.includes("halbelf") ||
    value.includes("half-elf") ||
    value.includes("elf") ||
    value.includes("mensch") ||
    value.includes("human") ||
    value.includes("tiefling") ||
    value.includes("half-orc") ||
    value.includes("halbork") ||
    value.includes("dragonborn")
  ) {
    return "30 ft";
  }
  return "";
}

function deriveArmorClassValue(sheetForm: CharacterSheetCanvasProps["sheetForm"], assignment: Record<AbilityKey, number>) {
  if (sheetForm.armor_class) {
    return sheetForm.armor_class;
  }
  const dexMod = abilityModifier(assignment.dexterity || 10);
  const equipment = `${sheetForm.starting_equipment} ${sheetForm.weapon_notes}`.toLowerCase();
  const spells = sheetForm.spells.toLowerCase();

  let armorBase = 10;
  let dexContribution = dexMod;
  if (equipment.includes("platten") || equipment.includes("plate")) {
    armorBase = 18;
    dexContribution = 0;
  } else if (equipment.includes("kettenpanzer") || equipment.includes("chain mail")) {
    armorBase = 16;
    dexContribution = 0;
  } else if (equipment.includes("schienen") || equipment.includes("splint")) {
    armorBase = 17;
    dexContribution = 0;
  } else if (equipment.includes("halbplatte") || equipment.includes("half plate")) {
    armorBase = 15;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("brustplatte") || equipment.includes("breastplate")) {
    armorBase = 14;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("schuppenpanzer") || equipment.includes("scale")) {
    armorBase = 14;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("kettenhemd") || equipment.includes("chain shirt")) {
    armorBase = 13;
    dexContribution = Math.min(2, dexMod);
  } else if (equipment.includes("beschlagene leder") || equipment.includes("studded leather")) {
    armorBase = 12;
  } else if (equipment.includes("leder") || equipment.includes("lederrüstung") || equipment.includes("lederruestung") || equipment.includes("leather armor")) {
    armorBase = 11;
  }
  if ((spells.includes("mage armor") || spells.includes("magische rüstung") || spells.includes("magische ruestung")) && armorBase < 13) {
    armorBase = 13;
    dexContribution = dexMod;
  }
  let total = armorBase + dexContribution;
  if (equipment.includes("schild") || equipment.includes("shield")) {
    total += 2;
  }
  return String(total);
}

function deriveSpellcastingAbility(classAndLevel: string): AbilityKey | null {
  switch (normalizeClassName(classAndLevel)) {
    case "bard":
    case "paladin":
    case "sorcerer":
    case "warlock":
      return "charisma";
    case "cleric":
    case "druid":
    case "ranger":
      return "wisdom";
    case "wizard":
      return "intelligence";
    default:
      return null;
  }
}

function parseStructuredRows(value: string, expectedColumns: number) {
  const lines = value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  const rows: ParsedTableRow[] = [];
  let pendingDescription = "";

  for (const line of lines) {
    const normalized = line.replace(/^beschreibung\s*:\s*/i, "").trim();
    if (/^beschreibung\s*:/i.test(line)) {
      if (rows.length > 0) {
        rows[rows.length - 1].description = normalized;
      } else {
        pendingDescription = normalized;
      }
      continue;
    }

    const columns = line.split("|").map((part) => part.trim());
    if (columns.length >= expectedColumns) {
      rows.push({
        columns: columns.slice(0, expectedColumns),
        description: pendingDescription,
      });
      pendingDescription = "";
      continue;
    }

    if (rows.length > 0) {
      rows[rows.length - 1].description = rows[rows.length - 1].description
        ? `${rows[rows.length - 1].description} ${line}`
        : line;
    } else {
      pendingDescription = pendingDescription ? `${pendingDescription} ${line}` : line;
    }
  }

  return rows;
}

type CharacterSheetCanvasProps = {
  sheetForm: SheetFormState;
  assignment: Record<AbilityKey, number>;
  builderSkillKeys: Set<string>;
  activeTab: SheetTab;
  onTabChange: (tab: SheetTab) => void;
  isEditMode: boolean;
  onFieldChange: (field: keyof SheetFormState, value: string) => void;
  onSave: () => void;
  saveDisabled: boolean;
  onToggleEditMode: () => void;
};

function CharacterSheetCanvas({
  sheetForm,
  assignment,
  builderSkillKeys,
  activeTab,
  onTabChange,
  isEditMode,
  onFieldChange,
  onSave,
  saveDisabled,
  onToggleEditMode,
}: CharacterSheetCanvasProps) {
  const { locale, tr } = useI18n();
  const proficiency = parseNumericText(sheetForm.proficiency_bonus) ?? deriveProficiencyBonus(sheetForm.class_and_level);
  const passivePerception = 10 + abilityModifier(assignment.wisdom || 10) + (builderSkillKeys.has("perception") ? proficiency : 0);
  const derivedArmorClass = deriveArmorClassValue(sheetForm, assignment);
  const derivedSpeed = sheetForm.speed || deriveBaseSpeed(sheetForm.race) || "—";
  const derivedHitDie = sheetForm.hit_dice || deriveHitDie(sheetForm.class_and_level) || "—";
  const derivedSavingThrows = sheetForm.saving_throw_proficiencies || deriveSavingThrowProficiencies(sheetForm.class_and_level) || "—";
  const spellcastingAbility = deriveSpellcastingAbility(sheetForm.class_and_level);
  const spellcastingModifier = spellcastingAbility ? abilityModifier(assignment[spellcastingAbility] || 10) : null;
  const derivedSpellSaveDC =
    spellcastingModifier === null ? "—" : String(8 + proficiency + spellcastingModifier);
  const derivedSpellAttackBonus =
    spellcastingModifier === null ? "—" : formatModifier(proficiency + spellcastingModifier);
  const combatRows = parseStructuredRows(sheetForm.combat_attacks, 7);
  const spellAttackRows = parseStructuredRows(sheetForm.spell_attacks, 7);
  const renderInlineField = (
    field: keyof SheetFormState,
    options?: { multiline?: boolean; displayFallback?: string; inputPlaceholder?: string }
  ) => {
    if (!isEditMode) {
      return sheetForm[field] || options?.displayFallback || "—";
    }
    if (options?.multiline) {
      return (
        <textarea
          className="sheet-inline-input sheet-inline-input--multiline"
          onChange={(event) => onFieldChange(field, event.target.value)}
          placeholder={options?.inputPlaceholder}
          rows={4}
          value={sheetForm[field]}
        />
      );
    }
    return (
      <input
        className="sheet-inline-input"
        onChange={(event) => onFieldChange(field, event.target.value)}
        placeholder={options?.inputPlaceholder}
        value={sheetForm[field]}
      />
    );
  };

  return (
    <section className="sheet-canvas">
      <div className="sheet-tabs">
        {([
          ["overview", tr("Overview", "Überblick")],
          ["abilities", tr("Abilities & Skills", "Attribute & Fertigkeiten")],
          ["combat", tr("Combat", "Kampf")],
          ["magic", tr("Magic", "Magie")],
          ["personality", tr("Personality", "Persönlichkeit")],
          ["gear", tr("Equipment & Magic", "Ausrüstung & Magie")],
        ] as Array<[SheetTab, string]>).map(([tab, label]) => (
          <button
            className={`sheet-tab${activeTab === tab ? " is-active" : ""}`}
            key={tab}
            onClick={() => onTabChange(tab)}
            type="button"
          >
            {label}
          </button>
        ))}
      </div>
      <div className="sheet-canvas__toolbar">
        <button className={`studio-button ${isEditMode ? "studio-button--primary" : "studio-button--ghost"}`} onClick={onToggleEditMode} type="button">
          <PenSquare size={16} />
          {isEditMode ? tr("Finish Editing", "Bearbeitung beenden") : tr("Edit", "Bearbeiten")}
        </button>
        {isEditMode ? (
          <button className="studio-button studio-button--ghost" disabled={saveDisabled} onClick={onSave} type="button">
            <Check size={16} />
            {tr("Save", "Speichern")}
          </button>
        ) : null}
      </div>
      <header className="sheet-canvas__header">
        <div className="sheet-canvas__name">
          <span>{tr("CHARACTER NAME", "CHARAKTERNAME")}</span>
          <strong>{renderInlineField("name", { displayFallback: tr("New Character", "Neuer Charakter") })}</strong>
        </div>
        <div className="sheet-canvas__identity">
          <article>
            <span>{tr("Class & Level", "Klasse & Stufe")}</span>
            <strong>{renderInlineField("class_and_level")}</strong>
          </article>
          <article>
            <span>{tr("Ancestry", "Volk")}</span>
            <strong>{renderInlineField("race")}</strong>
          </article>
          <article>
            <span>{tr("Background", "Hintergrund")}</span>
            <strong>{renderInlineField("background")}</strong>
          </article>
          <article>
            <span>{tr("Alignment", "Gesinnung")}</span>
            <strong>{renderInlineField("alignment")}</strong>
          </article>
          <article>
            <span>{tr("Player", "Spieler")}</span>
            <strong>{renderInlineField("player_name")}</strong>
          </article>
        </div>
      </header>

      {activeTab === "overview" ? (
        <div className="sheet-tab-panel">
          <div className="sheet-tab-grid sheet-tab-grid--overview">
            <section className="sheet-box sheet-box--story">
              <div className="sheet-box__title-row">
                <strong>{tr("Concept & Story", "Konzept & Geschichte")}</strong>
                <span>{tr("current builder state", "aktueller Stand aus dem Builder")}</span>
              </div>
              <p>{renderInlineField("concept", { multiline: true, displayFallback: tr("No clear concept entered yet.", "Noch kein klares Konzept eingetragen.") })}</p>
              <p>{renderInlineField("backstory", { multiline: true, displayFallback: tr("The backstory develops through the builder conversation.", "Die Hintergrundgeschichte wächst mit dem Builder-Dialog.") })}</p>
            </section>
            <section className="sheet-box">
              <strong>{tr("Basic Details", "Grunddaten")}</strong>
              <dl className="sheet-detail-list">
                <div><dt>{tr("Player", "Spieler")}</dt><dd>{renderInlineField("player_name")}</dd></div>
                <div><dt>{tr("Age", "Alter")}</dt><dd>{renderInlineField("age")}</dd></div>
                <div><dt>{tr("Size", "Größe")}</dt><dd>{renderInlineField("size")}</dd></div>
                <div><dt>{tr("Weight", "Gewicht")}</dt><dd>{renderInlineField("weight")}</dd></div>
                <div><dt>{tr("Eyes", "Augen")}</dt><dd>{renderInlineField("eyes")}</dd></div>
                <div><dt>{tr("Skin", "Haut")}</dt><dd>{renderInlineField("skin")}</dd></div>
                <div><dt>{tr("Hair", "Haare")}</dt><dd>{renderInlineField("hair")}</dd></div>
              </dl>
            </section>
          </div>
        </div>
      ) : null}

      {activeTab === "abilities" ? (
        <div className="sheet-tab-panel">
          <div className="sheet-tab-grid sheet-tab-grid--main-abilities">
            {abilityOrder.map((ability) => {
              const score = assignment[ability] || 0;
              return (
                <article className="sheet-ability" key={`canvas-${ability}`}>
                  <span>{locale === "de" ? abilityLabels[ability] : englishAbilityLabels[ability]}</span>
                  <strong>{score || "—"}</strong>
                  <em>{score ? formatModifier(abilityModifier(score)) : "—"}</em>
                </article>
              );
            })}
          </div>

          <div className="sheet-tab-grid sheet-tab-grid--ability-status">
            <section className="sheet-box">
              <strong>{tr("Ability Status", "Attributstatus")}</strong>
              <dl className="sheet-detail-list">
                <div><dt>{tr("Proficiency Bonus", "Übungsbonus")}</dt><dd>{formatModifier(proficiency)}</dd></div>
                <div><dt>{tr("Inspiration", "Inspiration")}</dt><dd>{renderInlineField("inspiration")}</dd></div>
                <div><dt>{tr("Passive Perception", "Passive Wahrnehmung")}</dt><dd>{passivePerception}</dd></div>
                <div><dt>{tr("Saving Throw Proficiencies", "Rettungswurf-Profizienzen")}</dt><dd>{derivedSavingThrows}</dd></div>
              </dl>
            </section>
            <section className="sheet-box">
              <strong>{tr("Derived Stats", "Abgeleitete Werte")}</strong>
              <dl className="sheet-detail-list">
                <div><dt>{tr("Armor Class", "Rüstungsklasse")}</dt><dd>{derivedArmorClass}</dd></div>
                <div><dt>{tr("Initiative", "Initiative")}</dt><dd>{assignment.dexterity ? formatModifier(abilityModifier(assignment.dexterity)) : "—"}</dd></div>
                <div><dt>{tr("Speed", "Bewegungsrate")}</dt><dd>{derivedSpeed}</dd></div>
                <div><dt>{tr("Hit Dice", "Trefferwürfel")}</dt><dd>{derivedHitDie}</dd></div>
              </dl>
            </section>
          </div>

          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>{tr("Skills", "Fertigkeiten")}</strong>
              <span>{tr("including calculated modifiers", "einschließlich berechneter Modifikatoren")}</span>
            </div>
            <div className="sheet-skills">
              {skillDefinitions.map((skill) => {
                const score = assignment[skill.ability] || 10;
                const isProficient = builderSkillKeys.has(skill.key);
                const total = abilityModifier(score) + (isProficient ? proficiency : 0);
                return (
                  <div className={`sheet-skill${isProficient ? " is-proficient" : ""}`} key={`canvas-skill-${skill.key}`}>
                    <div className="sheet-skill__label">
                      <span>{locale === "de" ? skill.label : englishSkillLabels[skill.key]}</span>
                    </div>
                    <em>{skill.ability.slice(0, 3).toUpperCase()}</em>
                    <strong>{formatModifier(total)}</strong>
                  </div>
                );
              })}
            </div>
          </section>
        </div>
      ) : null}

      {activeTab === "combat" ? (
        <div className="sheet-tab-panel">
          <section className="sheet-box sheet-box--combat">
            <article>
              <span>{tr("Armor Class", "Rüstungsklasse")}</span>
              <strong>{renderInlineField("armor_class")}</strong>
            </article>
            <article>
              <span>{tr("Initiative", "Initiative")}</span>
              <strong>{assignment.dexterity ? formatModifier(abilityModifier(assignment.dexterity)) : "—"}</strong>
            </article>
            <article>
              <span>{tr("Speed", "Bewegung")}</span>
              <strong>{renderInlineField("speed", { displayFallback: derivedSpeed })}</strong>
            </article>
            <article>
              <span>{tr("Max HP", "TP max")}</span>
              <strong>{renderInlineField("hit_point_max")}</strong>
            </article>
            <article>
              <span>{tr("Current HP", "Aktuelle TP")}</span>
              <strong>{renderInlineField("current_hit_points", { displayFallback: sheetForm.hit_point_max || "—" })}</strong>
            </article>
            <article>
              <span>{tr("Temp. HP", "Temp. TP")}</span>
              <strong>{renderInlineField("temporary_hit_points", { displayFallback: "0" })}</strong>
            </article>
            <article>
              <span>{tr("Hit Dice", "Trefferwürfel")}</span>
              <strong>{renderInlineField("hit_dice", { displayFallback: derivedHitDie })}</strong>
            </article>
            <article>
              <span>{tr("Proficiency Bonus", "Übungsbonus")}</span>
              <strong>{formatModifier(proficiency)}</strong>
            </article>
            <article>
              <span>{tr("Inspiration", "Inspiration")}</span>
              <strong>{sheetForm.inspiration || "—"}</strong>
            </article>
            <article>
              <span>{tr("Passive Perception", "Passive Wahrnehmung")}</span>
              <strong>{passivePerception}</strong>
            </article>
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>{tr("Combat Equipment", "Kampfausrüstung")}</strong>
              <span>{tr("items directly relevant in combat", "im Kampf direkt relevante Gegenstände")}</span>
            </div>
            {renderInlineField("combat_overview", {
              multiline: true,
              inputPlaceholder: "Rüstung, Schild, Waffen, relevante Gegenstände und Kampfstil.",
            })}
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>{tr("Attacks", "Angriffe")}</strong>
              <span>{tr("ATTACK · PROF. · ABILITY · RANGE · BONUS · DAMAGE · DAMAGE TYPE", "ANGRIFF · ÜB · ATTR. · REICHWEITE · BONUS · SCHADEN · SCHADENTYP")}</span>
            </div>
            <div className="sheet-table-card">
              {isEditMode ? (
                <div className="sheet-table__body-copy">
                  {renderInlineField("combat_attacks", {
                    multiline: true,
                    inputPlaceholder:
                      tr("Example:\nLongbow | +2 | DEX | 150/600 ft | +5 | 1d8+3 | Piercing\nDescription: Standard ranged weapon.", "Beispiel:\nLangbogen | +2 | GES | 150/600 ft | +5 | 1d8+3 | Stich\nBeschreibung: Standard-Fernkampfwaffe."),
                  })}
                </div>
              ) : combatRows.length > 0 ? (
                <div className="sheet-table-list">
                  <div className="sheet-table sheet-table--combat">
                    <div className="sheet-table__head">{tr("Attack", "Angriff")}</div>
                    <div className="sheet-table__head">{tr("Prof.", "ÜB")}</div>
                    <div className="sheet-table__head">{tr("Ability", "Attr.")}</div>
                    <div className="sheet-table__head">{tr("Range", "Reichweite")}</div>
                    <div className="sheet-table__head">{tr("Bonus", "Bonus")}</div>
                    <div className="sheet-table__head">{tr("Damage", "Schaden")}</div>
                    <div className="sheet-table__head">{tr("Damage Type", "Schadentyp")}</div>
                  </div>
                  {combatRows.map((row, index) => (
                    <div className="sheet-table-entry" key={`combat-row-${index}`}>
                      <div className="sheet-table sheet-table--combat sheet-table--values">
                        {row.columns.map((column, columnIndex) => (
                          <div className="sheet-table__cell" key={`combat-${index}-${columnIndex}`}>{column || "—"}</div>
                        ))}
                      </div>
                      <div className="sheet-table-entry__description">
                        <span>{tr("Description", "Beschreibung")}</span>
                        <p>{row.description || "—"}</p>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="sheet-table__body-copy">
                  <p className="muted-copy">{tr("No attacks entered yet.", "Noch keine Angriffe eingetragen.")}</p>
                </div>
              )}
            </div>
          </section>
        </div>
      ) : null}

      {activeTab === "magic" ? (
        <div className="sheet-tab-panel">
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>{tr("Magic Status", "Magiestatus")}</strong>
              <span>{tr("spell slots, DC, and spell attack bonus", "Zauberplätze, SG und Zauberangriffsbonus")}</span>
            </div>
            <div className="sheet-tab-grid sheet-tab-grid--ability-status">
              <dl className="sheet-detail-list">
                <div><dt>{tr("Spells", "Zauber")}</dt><dd>{renderInlineField("spells", { multiline: true, inputPlaceholder: tr("Spell slots or available magic.", "Zauberplätze oder verfügbare Magie.") })}</dd></div>
              </dl>
              <dl className="sheet-detail-list">
                <div><dt>{tr("Spell Save DC", "Zauberrettungswurf-SG")}</dt><dd>{renderInlineField("spell_save_dc", { displayFallback: derivedSpellSaveDC })}</dd></div>
                <div><dt>{tr("Spell Attack Bonus", "Zauberangriffsbonus")}</dt><dd>{renderInlineField("spell_attack_bonus", { displayFallback: derivedSpellAttackBonus })}</dd></div>
              </dl>
            </div>
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>{tr("Spell Attacks", "Zauberangriffe")}</strong>
              <span>{tr("LEVEL · ATTACK · ABILITY · RANGE · BONUS · DAMAGE · DAMAGE TYPE", "STUFE · ANGRIFF · ATTR. · REICHWEITE · BONUS · SCHADEN · SCHADENTYP")}</span>
            </div>
            <div className="sheet-table-card">
              {isEditMode ? (
                <div className="sheet-table__body-copy">
                  {renderInlineField("spell_attacks", {
                    multiline: true,
                    inputPlaceholder:
                      tr("Example:\nCantrip | Fire Bolt | CHA | 120 ft | +5 | 1d10 | Fire\nDescription: Ranged spell attack.", "Beispiel:\nZaubertrick | Feuerstrahl | CHA | 120 ft | +5 | 1d10 | Feuer\nBeschreibung: Fernzauberangriff."),
                  })}
                </div>
              ) : spellAttackRows.length > 0 ? (
                <div className="sheet-table-list">
                  <div className="sheet-table sheet-table--combat">
                    <div className="sheet-table__head">{tr("Level", "Stufe")}</div>
                    <div className="sheet-table__head">{tr("Attack", "Angriff")}</div>
                    <div className="sheet-table__head">{tr("Ability", "Attr.")}</div>
                    <div className="sheet-table__head">{tr("Range", "Reichweite")}</div>
                    <div className="sheet-table__head">{tr("Bonus", "Bonus")}</div>
                    <div className="sheet-table__head">{tr("Damage", "Schaden")}</div>
                    <div className="sheet-table__head">{tr("Damage Type", "Schadentyp")}</div>
                  </div>
                  {spellAttackRows.map((row, index) => (
                    <div className="sheet-table-entry" key={`spell-row-${index}`}>
                      <div className="sheet-table sheet-table--combat sheet-table--values">
                        {row.columns.map((column, columnIndex) => (
                          <div className="sheet-table__cell" key={`spell-${index}-${columnIndex}`}>{column || "—"}</div>
                        ))}
                      </div>
                      <div className="sheet-table-entry__description">
                        <span>{tr("Description", "Beschreibung")}</span>
                        <p>{row.description || "—"}</p>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="sheet-table__body-copy">
                  <p className="muted-copy">{tr("No spell attacks entered yet.", "Noch keine Zauberangriffe eingetragen.")}</p>
                </div>
              )}
            </div>
          </section>
          <section className="sheet-box">
            <div className="sheet-box__title-row">
              <strong>{tr("Additional Spells", "Weitere Zauber")}</strong>
              <span>{tr("short descriptions for other spells", "Kurzbeschreibungen für andere Zauber")}</span>
            </div>
            {renderInlineField("spell_notes", {
              multiline: true,
              inputPlaceholder: "Wirkung, Konzentration, Dauer, taktische Hinweise oder Kurzbeschreibung.",
            })}
          </section>
        </div>
      ) : null}

      {activeTab === "personality" ? (
        <div className="sheet-tab-panel">
          <div className="sheet-tab-grid sheet-tab-grid--overview">
            <section className="sheet-box">
              <strong>{tr("Personality", "Persönlichkeit")}</strong>
              <dl className="sheet-detail-list">
                <div><dt>{tr("Traits", "Merkmale")}</dt><dd>{renderInlineField("personality_traits", { multiline: true })}</dd></div>
                <div><dt>{tr("Ideals", "Ideale")}</dt><dd>{renderInlineField("ideals", { multiline: true })}</dd></div>
                <div><dt>{tr("Bonds", "Bindungen")}</dt><dd>{renderInlineField("bonds", { multiline: true })}</dd></div>
                <div><dt>{tr("Flaws", "Makel")}</dt><dd>{renderInlineField("flaws", { multiline: true })}</dd></div>
              </dl>
            </section>
            <section className="sheet-box">
              <strong>{tr("Body & Appearance", "Körper & Auftreten")}</strong>
              <dl className="sheet-detail-list">
                <div><dt>{tr("Age", "Alter")}</dt><dd>{renderInlineField("age")}</dd></div>
                <div><dt>{tr("Size", "Größe")}</dt><dd>{renderInlineField("size")}</dd></div>
                <div><dt>{tr("Weight", "Gewicht")}</dt><dd>{renderInlineField("weight")}</dd></div>
                <div><dt>{tr("Eyes", "Augen")}</dt><dd>{renderInlineField("eyes")}</dd></div>
                <div><dt>{tr("Skin", "Haut")}</dt><dd>{renderInlineField("skin")}</dd></div>
                <div><dt>{tr("Hair", "Haare")}</dt><dd>{renderInlineField("hair")}</dd></div>
                <div><dt>{tr("Languages", "Sprachen")}</dt><dd>{renderInlineField("languages", { multiline: true, displayFallback: "—" })}</dd></div>
                <div><dt>{tr("Senses", "Sinne")}</dt><dd>{renderInlineField("senses")}</dd></div>
              </dl>
            </section>
          </div>
        </div>
      ) : null}

      {activeTab === "gear" ? (
        <div className="sheet-tab-panel">
          <section className="sheet-box">
            <strong>{tr("Equipment", "Ausrüstung")}</strong>
            <dl className="sheet-detail-list">
              <div><dt>{tr("Starting Equipment", "Startausrüstung")}</dt><dd>{renderInlineField("starting_equipment", { multiline: true })}</dd></div>
              <div><dt>{tr("Money", "Geld")}</dt><dd>{renderInlineField("starting_money", { inputPlaceholder: tr("e.g. 15 gp, 7 sp", "z. B. 15 GM, 7 SM") })}</dd></div>
              <div><dt>{tr("Current Money", "Aktuelles Geld")}</dt><dd>{renderInlineField("current_money", { inputPlaceholder: tr("e.g. 22 gp, 4 sp", "z. B. 22 GM, 4 SM") })}</dd></div>
              <div><dt>{tr("Current Inventory", "Aktuelles Inventar")}</dt><dd>{renderInlineField("current_inventory", { multiline: true, inputPlaceholder: tr("Found, purchased, or traded items.", "Gefundene, gekaufte oder gehandelte Gegenstände.") })}</dd></div>
              <div><dt>{tr("Level-up Available", "Stufenaufstieg bereit")}</dt><dd>{renderInlineField("level_up_available")}</dd></div>
              <div><dt>{tr("Tools", "Werkzeuge")}</dt><dd>{renderInlineField("tools_and_proficiencies", { multiline: true })}</dd></div>
              <div><dt>{tr("Weapon Notes", "Waffennotizen")}</dt><dd>{renderInlineField("weapon_notes", { multiline: true })}</dd></div>
              <div><dt>{tr("Allies", "Verbündete")}</dt><dd>{renderInlineField("allies", { multiline: true })}</dd></div>
              <div><dt>{tr("XP", "EP")}</dt><dd>{renderInlineField("experience_points")}</dd></div>
            </dl>
          </section>
        </div>
      ) : null}
    </section>
  );
}

export function CharactersScreen({ characters, campaigns, documents, initialBuilderSeed }: CharactersScreenProps) {
  const router = useRouter();
  const { locale, tr } = useI18n();
  const { notify } = useNotifications();
  const statusLabel = (status: string) => ({
    draft: tr("draft", "Entwurf"), ready: tr("ready", "bereit"), assigned: tr("assigned", "zugeordnet"),
    idle: tr("idle", "bereit"), error: tr("error", "Fehler"), unsupported: tr("unsupported", "nicht unterstützt"),
  }[status] ?? status);
  const [rulesetFilter, setRulesetFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [rosterCharacters, setRosterCharacters] = useState<Character[]>(characters);
  const [isBuilderOpen, setIsBuilderOpen] = useState(false);
  const [builderStep, setBuilderStep] = useState<BuilderStep>("start");
  const [builderCharacter, setBuilderCharacter] = useState<Character | null>(null);
  const [builderMessages, setBuilderMessages] = useState<CharacterBuilderMessage[]>([]);
  const [builderInput, setBuilderInput] = useState("");
  const [isBuilderResponding, setIsBuilderResponding] = useState(false);
  const [builderSpeechLoadingKey, setBuilderSpeechLoadingKey] = useState("");
  const [builderSpeechActiveKey, setBuilderSpeechActiveKey] = useState("");
  const [builderSpeechUrl, setBuilderSpeechUrl] = useState("");
  const [builderSpeechError, setBuilderSpeechError] = useState<string | null>(null);
  const [builderReferencePopup, setBuilderReferencePopup] = useState<null | {
    documentId: string;
    documentName: string;
    page: number | null;
  }>(null);
  const [builderSTTError, setBuilderSTTError] = useState<string | null>(null);
  const [builderSTTStatus, setBuilderSTTStatus] = useState("");
  const [isBuilderRecording, setIsBuilderRecording] = useState(false);
  const [isBuilderTranscribing, setIsBuilderTranscribing] = useState(false);
  const [builderCharacterName, setBuilderCharacterName] = useState("");
  const [builderPlayerName, setBuilderPlayerName] = useState("");
  const [selectedRulesetKey, setSelectedRulesetKey] = useState("");
  const [selectedDocumentIds, setSelectedDocumentIds] = useState<string[]>([]);
  const [selectedCampaignId, setSelectedCampaignId] = useState("");
  const [isRollModalOpen, setIsRollModalOpen] = useState(false);
  const [rollCameraStatus, setRollCameraStatus] = useState<"idle" | "ready" | "error" | "unsupported">("idle");
  const [rollCameraMessage, setRollCameraMessage] = useState(() => tr("Camera not started yet.", "Kamera noch nicht gestartet."));
  const [isRollCapturing, setIsRollCapturing] = useState(false);
  const [currentRollIndex, setCurrentRollIndex] = useState(0);
  const [confirmedRolls, setConfirmedRolls] = useState<boolean[]>(() => Array.from({ length: guidedRollCount }, () => false));
  const [activeSheetTab, setActiveSheetTab] = useState<SheetTab>("overview");
  const [isSheetEditMode, setIsSheetEditMode] = useState(false);
  const [sheetForm, setSheetForm] = useState<SheetFormState>({
    name: "",
    player_name: "",
    class_and_level: "",
    race: "",
    background: "",
    alignment: "",
    languages: "",
    armor_class: "",
    hit_point_max: "",
    current_hit_points: "",
    temporary_hit_points: "",
    speed: "",
    proficiency_bonus: "",
    inspiration: "",
    experience_points: "",
    hit_dice: "",
    concept: "",
    builder_stage: "",
    personality_traits: "",
    ideals: "",
    bonds: "",
    flaws: "",
    backstory: "",
    age: "",
    size: "",
    weight: "",
    eyes: "",
    skin: "",
    hair: "",
    allies: "",
    senses: "",
    tools_and_proficiencies: "",
    weapon_notes: "",
    starting_money: "",
    current_money: "",
    current_inventory: "",
    level_up_available: "",
    combat_overview: "",
    combat_attacks: "",
    skill_proficiencies: "",
    saving_throw_proficiencies: "",
    starting_equipment: "",
    spells: "",
    spell_save_dc: "",
    spell_attack_bonus: "",
    spell_attacks: "",
    spell_notes: "",
  });
  const [abilityMethod, setAbilityMethod] = useState<AbilityMethod>("standard");
  const [rolledSetsText, setRolledSetsText] = useState("6, 6, 5, 2\n6, 5, 4, 3\n6, 5, 5, 1\n6, 4, 4, 3\n5, 5, 4, 2\n6, 4, 3, 2\n5, 4, 3, 1");
  const [resolvedValues, setResolvedValues] = useState<number[]>([]);
  const [assignment, setAssignment] = useState<Record<AbilityKey, number>>(createEmptyAssignment());
  const [ruleSummary, setRuleSummary] = useState("");
  const [validationMessage, setValidationMessage] = useState("");
  const [assignmentConfirmed, setAssignmentConfirmed] = useState(false);
  const [isPending, startTransition] = useTransition();
  const transcriptEndRef = useRef<HTMLDivElement | null>(null);
  const builderSpeechObjectUrlRef = useRef<string | null>(null);
  const builderSpeechAudioRef = useRef<HTMLAudioElement | null>(null);
  const builderSpeechRequestKeyRef = useRef("");
  const lastAutoPlayedBuilderSpeechKeyRef = useRef("");
  const builderRecorderRef = useRef<MediaRecorder | null>(null);
  const builderStopTimerRef = useRef<number | null>(null);
  const builderAudioStreamRef = useRef<MediaStream | null>(null);
  const builderRecordedChunksRef = useRef<Blob[]>([]);
  const rollVideoRef = useRef<HTMLVideoElement | null>(null);
  const rollCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const rollStreamRef = useRef<MediaStream | null>(null);
  const seedAppliedRef = useRef(false);

  useEffect(() => {
    setRosterCharacters(characters);
  }, [characters]);

  function upsertRosterCharacter(updated: Character) {
    setRosterCharacters((current) => {
      const index = current.findIndex((item) => item.id === updated.id);
      if (index === -1) {
        return [updated, ...current];
      }
      const next = [...current];
      next[index] = updated;
      return next;
    });
  }

  function removeRosterCharacter(characterId: string) {
    setRosterCharacters((current) => current.filter((item) => item.id !== characterId));
  }

  const rulesDocuments = useMemo(
    () =>
      documents.filter(
        (document) => document.type === "rules" && !String(document.metadata?.kind ?? "").endsWith("_guide")
      ),
    [documents]
  );
  const rulesetGroups = useMemo<RulesetGroup[]>(() => {
    const groups = new Map<string, RulesetGroup>();
    for (const document of rulesDocuments) {
      const ruleset = deriveRuleset(document.metadata);
      const key = `${ruleset.work}::${ruleset.version}`;
      const current = groups.get(key) ?? { key, work: ruleset.work, version: ruleset.version, documents: [] };
      current.documents.push(document);
      groups.set(key, current);
    }
    return [...groups.values()].sort((left, right) => left.key.localeCompare(right.key));
  }, [rulesDocuments]);

  const roster = useMemo<RosterCharacter[]>(
    () =>
      rosterCharacters.map((character) => {
        const ruleset = deriveRuleset(character.metadata);
        return {
          ...character,
          rulesetLabel: `${ruleset.work} ${ruleset.version}`,
          statusLabel: safeString(character.metadata.builder_status) || "ready",
          concept: safeString(character.metadata.concept),
          selectedDocumentNames: splitMetadataList(character.metadata.selected_document_names),
        };
      }),
    [rosterCharacters]
  );

  const availableRulesetFilters = useMemo(
    () => ["all", ...new Set(roster.map((character) => character.rulesetLabel).sort((left, right) => left.localeCompare(right)))],
    [roster]
  );

  const filteredCharacters = useMemo(
    () =>
      roster.filter((character) => {
        if (rulesetFilter !== "all" && character.rulesetLabel !== rulesetFilter) {
          return false;
        }
        if (statusFilter !== "all" && character.statusLabel !== statusFilter) {
          return false;
        }
        return true;
      }),
    [roster, rulesetFilter, statusFilter]
  );

  const selectedRulesetGroup = useMemo(
    () => rulesetGroups.find((group) => group.key === selectedRulesetKey) ?? rulesetGroups[0] ?? null,
    [rulesetGroups, selectedRulesetKey]
  );
  const builderReferenceDocument = useMemo(() => {
    if (!builderReferencePopup) {
      return null;
    }
    return documents.find((document) => document.id === builderReferencePopup.documentId) ?? null;
  }, [builderReferencePopup, documents]);
  const builderReferenceFileUrl = builderReferenceDocument
    ? `${apiBaseUrl}/api/documents/${builderReferenceDocument.id}/file${
        builderReferencePopup?.page ? `#page=${builderReferencePopup.page}` : ""
      }`
    : "";

  const builderSkillKeys = useMemo(
    () => new Set(splitMetadataList(sheetForm.skill_proficiencies).map(normalizeSkillToken)),
    [sheetForm.skill_proficiencies]
  );

  useEffect(() => {
    if (rollCameraStatus === "idle") {
      setRollCameraMessage(tr("Camera not started yet.", "Kamera noch nicht gestartet."));
    }
  }, [locale, rollCameraStatus, tr]);

  useEffect(() => {
    if (!selectedRulesetKey && rulesetGroups[0]) {
      setSelectedRulesetKey(rulesetGroups[0].key);
    }
  }, [rulesetGroups, selectedRulesetKey]);

  useEffect(() => {
    if (!initialBuilderSeed || seedAppliedRef.current || rulesetGroups.length === 0) {
      return;
    }
    seedAppliedRef.current = true;
    openNewBuilder();
    setBuilderCharacterName(initialBuilderSeed.characterName ?? "");
    setBuilderPlayerName(initialBuilderSeed.playerName);
    setSelectedCampaignId(initialBuilderSeed.campaignId);
    const matchingRuleset = rulesetGroups.find(
      (group) => group.work === initialBuilderSeed.rulesetWork && group.version === initialBuilderSeed.rulesetVersion
    );
    if (matchingRuleset) {
      setSelectedRulesetKey(matchingRuleset.key);
    }
    notify({
      title: "Character Builder",
      message: "Builder wurde aus dem Player Portal vorbefuellt geoeffnet.",
      tone: "info",
    });
  }, [initialBuilderSeed, notify, rulesetGroups]);

  useEffect(() => {
    if (!selectedRulesetGroup) {
      setSelectedDocumentIds([]);
      return;
    }
    setSelectedDocumentIds((current) => {
      const validIds = current.filter((id) => selectedRulesetGroup.documents.some((document) => document.id === id));
      if (validIds.length > 0) {
        return validIds;
      }
      return selectedRulesetGroup.documents.map((document) => document.id);
    });
  }, [selectedRulesetGroup]);

  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [builderMessages, isBuilderResponding]);

  useEffect(() => {
    if (builderStep !== "chat" || isBuilderResponding || builderSpeechLoadingKey) {
      return;
    }
    const latestAssistantMessage = [...builderMessages]
      .reverse()
      .find((message) => message.role === "assistant" && message.content.trim());
    if (!latestAssistantMessage) {
      return;
    }
    const speechKey = `${latestAssistantMessage.created_at}:${latestAssistantMessage.content}`;
    if (lastAutoPlayedBuilderSpeechKeyRef.current === speechKey) {
      return;
    }
    lastAutoPlayedBuilderSpeechKeyRef.current = speechKey;
    void handlePlayBuilderSpeech(latestAssistantMessage);
  }, [builderMessages, builderSpeechLoadingKey, builderStep, isBuilderResponding]);

  useEffect(() => {
    return () => {
      builderRecorderRef.current?.stop();
      builderAudioStreamRef.current?.getTracks().forEach((track) => track.stop());
      if (builderSpeechObjectUrlRef.current) {
        URL.revokeObjectURL(builderSpeechObjectUrlRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (
      isBuilderOpen &&
      builderStep === "chat" &&
      safeString(builderCharacter?.metadata.creation_method) === "rolled" &&
      safeString(builderCharacter?.metadata.builder_stage) === "ability_scores" &&
      builderCharacter?.metadata &&
      !hasResolvedRollData(builderCharacter.metadata)
    ) {
      setIsRollModalOpen(true);
    }
  }, [builderCharacter, builderStep, isBuilderOpen]);

  useEffect(() => {
    if (!isRollModalOpen) {
      if (rollStreamRef.current) {
        rollStreamRef.current.getTracks().forEach((track) => track.stop());
        rollStreamRef.current = null;
      }
      if (rollVideoRef.current) {
        rollVideoRef.current.srcObject = null;
      }
      setRollCameraStatus("idle");
      setRollCameraMessage(tr("Camera not started yet.", "Kamera noch nicht gestartet."));
    }
  }, [isRollModalOpen]);

  function syncSheetForm(character: Character) {
    setSheetForm({
      name: character.name,
      player_name: character.player_name,
      class_and_level: character.class_and_level,
      race: character.race,
      background: character.background,
      alignment: character.alignment,
      languages: character.languages.join(", "),
      armor_class: character.armor_class != null ? String(character.armor_class) : "",
      hit_point_max: character.hit_point_max != null ? String(character.hit_point_max) : "",
      current_hit_points: safeString(character.metadata.current_hit_points),
      temporary_hit_points: safeString(character.metadata.temporary_hit_points),
      speed: character.speed,
      proficiency_bonus: character.proficiency_bonus,
      inspiration: safeString(character.metadata.inspiration),
      experience_points: safeString(character.metadata.experience_points),
      hit_dice: safeString(character.metadata.hit_dice),
      concept: safeString(character.metadata.concept),
      builder_stage: safeString(character.metadata.builder_stage),
      personality_traits: safeString(character.metadata.personality_traits),
      ideals: safeString(character.metadata.ideals),
      bonds: safeString(character.metadata.bonds),
      flaws: safeString(character.metadata.flaws),
      backstory: safeString(character.metadata.backstory),
      age: safeString(character.metadata.age),
      size: safeString(character.metadata.size),
      weight: safeString(character.metadata.weight),
      eyes: safeString(character.metadata.eyes),
      skin: safeString(character.metadata.skin),
      hair: safeString(character.metadata.hair),
      allies: safeString(character.metadata.allies),
      senses: safeString(character.metadata.senses),
      tools_and_proficiencies: metadataListToText(character.metadata.tools_and_proficiencies),
      weapon_notes: metadataListToText(character.metadata.weapon_notes),
      starting_money: safeString(character.metadata.starting_money),
      current_money: safeString(character.metadata.current_money),
      current_inventory: metadataListToText(character.metadata.current_inventory),
      level_up_available: safeString(character.metadata.level_up_available),
      combat_overview: safeString(character.metadata.combat_overview),
      combat_attacks: metadataStructuredText(character.metadata.combat_attacks),
      skill_proficiencies: metadataListToText(character.metadata.skill_proficiencies),
      saving_throw_proficiencies: metadataListToText(character.metadata.saving_throw_proficiencies),
      starting_equipment: metadataListToText(character.metadata.starting_equipment),
      spells: metadataListToText(character.metadata.spells),
      spell_save_dc: safeString(character.metadata.spell_save_dc),
      spell_attack_bonus: safeString(character.metadata.spell_attack_bonus),
      spell_attacks: metadataStructuredText(character.metadata.spell_attacks),
      spell_notes: safeString(character.metadata.spell_notes),
    });
    setAbilityMethod((safeString(character.metadata.creation_method) as AbilityMethod) || "standard");
    const metadataRolledSets = character.metadata.rolled_sets;
    if (Array.isArray(metadataRolledSets) && metadataRolledSets.length > 0) {
      const normalized = metadataRolledSets
        .map((set) => (Array.isArray(set) ? set.map((value) => Number(value) || 0) : []))
        .filter((set) => set.length > 0)
        .map((set) => {
          const next = [...set].slice(0, 4);
          while (next.length < 4) next.push(0);
          return next;
        });
      if (normalized.length > 0) {
        setRolledSetsText(normalized.map((set) => set.join(", ")).join("\n"));
        setConfirmedRolls(
          Array.from({ length: guidedRollCount }, (_, index) => {
            const set = normalized[index];
            return Boolean(set) && set.every((value) => value >= 1 && value <= 6);
          })
        );
      }
    }
    const storedAssignment = normalizeAssignment(character.abilities);
    const hasStoredAbilities = Object.values(storedAssignment).some((value) => value > 0);
    setAssignment(hasStoredAbilities ? storedAssignment : normalizeSuggestedAssignment(character.metadata.suggested_assignment));
  }

  function openNewBuilder() {
    setBuilderCharacter(null);
    setBuilderMessages([]);
    setBuilderInput("");
    setIsBuilderResponding(false);
    setBuilderCharacterName("");
    setBuilderPlayerName("");
    setSelectedCampaignId("");
    setIsRollModalOpen(false);
    setCurrentRollIndex(0);
    setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
    setBuilderStep("start");
    setActiveSheetTab("overview");
    setResolvedValues([]);
    setAssignment(createEmptyAssignment());
    setAssignmentConfirmed(false);
    setValidationMessage("");
    setRuleSummary("");
    setIsBuilderOpen(true);
  }

  function openBuilderForCharacter(character: RosterCharacter) {
    const freshestCharacter = rosterCharacters.find((item) => item.id === character.id) ?? character;
    setBuilderCharacter(freshestCharacter);
    setBuilderMessages(parseBuilderMessages(freshestCharacter.metadata));
    setBuilderInput("");
    setIsBuilderResponding(false);
    setBuilderReferencePopup(null);
    setBuilderCharacterName(freshestCharacter.name);
    setBuilderPlayerName(freshestCharacter.player_name);
    setIsRollModalOpen(false);
    setCurrentRollIndex(0);
    setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
    syncSheetForm(freshestCharacter);
    setBuilderStep("chat");
    setActiveSheetTab("overview");
    setIsBuilderOpen(true);
  }

  function closeBuilder() {
    setIsBuilderOpen(false);
    setBuilderStep("start");
    setBuilderInput("");
    setIsBuilderResponding(false);
    setBuilderReferencePopup(null);
    setIsRollModalOpen(false);
    setCurrentRollIndex(0);
    setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
    setValidationMessage("");
    setRuleSummary("");
  }

  function currentRollSets() {
    return ensureRolledSets(rolledSetsText);
  }

  function updateRolledSet(rowIndex: number, dieIndex: number, value: number) {
    const sets = currentRollSets();
    sets[rowIndex][dieIndex] = value;
    setRolledSetsText(sets.map((set) => set.join(", ")).join("\n"));
  }

  async function handleStartRollCamera() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setRollCameraStatus("unsupported");
      setRollCameraMessage(tr("This browser does not support camera access.", "Dieser Browser unterstützt keinen Kamerazugriff."));
      return;
    }
    try {
      if (rollStreamRef.current) {
        rollStreamRef.current.getTracks().forEach((track) => track.stop());
      }
      const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
      rollStreamRef.current = stream;
      if (rollVideoRef.current) {
        rollVideoRef.current.srcObject = stream;
      }
      setRollCameraStatus("ready");
      setRollCameraMessage(tr("Camera active. Hold exactly 4d6 in view for the current roll.", "Kamera aktiv. Halte jetzt genau 4d6 für den aktuellen Wurf ins Bild."));
    } catch (error) {
      setRollCameraStatus("error");
      setRollCameraMessage(error instanceof Error ? error.message : tr("Could not start camera.", "Kamera konnte nicht gestartet werden."));
    }
  }

  function captureRollFrame() {
    const video = rollVideoRef.current;
    const canvas = rollCanvasRef.current;
    if (!video || !canvas || video.videoWidth === 0 || video.videoHeight === 0) {
      return null;
    }
    const width = video.videoWidth;
    const height = video.videoHeight;
    const cropWidth = Math.round(width * 0.86);
    const cropHeight = Math.round(height * 0.86);
    const cropX = Math.max(0, Math.round((width - cropWidth) / 2));
    const cropY = Math.max(0, Math.round((height - cropHeight) / 2));
    const targetWidth = 1280;
    const targetHeight = Math.round((cropHeight / cropWidth) * targetWidth);
    canvas.width = targetWidth;
    canvas.height = targetHeight;
    const context = canvas.getContext("2d");
    if (!context) {
      return null;
    }
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = "high";
    context.drawImage(video, cropX, cropY, cropWidth, cropHeight, 0, 0, canvas.width, canvas.height);
    return canvas.toDataURL("image/jpeg", 0.92);
  }

  async function handleDetectCurrentRoll() {
    if (rollCameraStatus !== "ready") {
      setRollCameraMessage(tr("Start the camera for the dice step first.", "Bitte starte zuerst die Kamera für den Würfelschritt."));
      return;
    }
    const imageDataUrl = captureRollFrame();
    if (!imageDataUrl) {
      setRollCameraMessage(tr("No camera image could be read.", "Es konnte gerade kein Kamerabild gelesen werden."));
      return;
    }
    setIsRollCapturing(true);
    setRollCameraMessage(tr(`Evaluating values for roll ${currentRollIndex + 1}...`, `Werte für Wurf ${currentRollIndex + 1} werden ausgewertet …`));
    try {
      const response = await detectDiceFromImage({ image_data_url: imageDataUrl, language: locale });
      const detected = response.dice
        .filter((die) => die.type.toLowerCase() === "d6")
        .map((die) => die.value)
        .filter((value) => value >= 1 && value <= 6)
        .slice(0, 4);
      if (detected.length === 0) {
        setRollCameraMessage(response.notes || tr("No clear d6 detected. Reposition them and evaluate again.", "Keine klaren d6 erkannt. Bitte neu legen und erneut auswerten."));
        return;
      }
      while (detected.length < 4) {
        detected.push(1);
      }
      const sets = currentRollSets();
      sets[currentRollIndex] = detected;
      setRolledSetsText(sets.map((set) => set.join(", ")).join("\n"));
      setConfirmedRolls((current) => current.map((value, index) => (index === currentRollIndex ? false : value)));
      setRollCameraMessage(
        tr(`Roll ${currentRollIndex + 1} detected: ${detected.join(", ")}. Check and correct the values if needed.`, `Wurf ${currentRollIndex + 1} erkannt: ${detected.join(", ")}. Bitte kurz prüfen und bei Bedarf korrigieren.`)
      );
    } catch (error) {
      setRollCameraMessage(error instanceof Error ? error.message : tr("Camera evaluation failed.", "Kameraauswertung fehlgeschlagen."));
    } finally {
      setIsRollCapturing(false);
    }
  }

  function handleRollSetChange(rowIndex: number, dieIndex: number, value: number) {
    updateRolledSet(rowIndex, dieIndex, value);
    setConfirmedRolls((current) => current.map((entry, index) => (index === rowIndex ? false : entry)));
  }

  function finalizeRollSequence(sets: number[][]) {
    if (!builderCharacter) {
      return;
    }
    setIsRollModalOpen(false);
    setIsBuilderResponding(true);
    setRollCameraMessage(tr("All six rolls are confirmed. The AI is applying the values and continuing.", "Alle sechs Würfe sind bestätigt. Die KI übernimmt jetzt die Werte und macht weiter."));
    startTransition(async () => {
      try {
        const resolved = await resolveAbilityScores({
          method: "rolled",
          class: sheetForm.class_and_level,
          rolled_sets: sets,
        });
        setResolvedValues(resolved.values);
        setAssignment(normalizeAssignment(resolved.assignment));
        setRuleSummary(resolved.rule_summary);
        setAssignmentConfirmed(false);
        setValidationMessage(tr("The six rolls were applied. The AI will guide you through the next step.", "Die sechs Würfe wurden übernommen. Die KI führt dich jetzt durch den nächsten Schritt."));

        const updatedDraft = await applyCharacterBuilderPatch(builderCharacter.id, {
          patch: {
            metadata: {
              creation_method: "rolled",
              builder_stage: "ability_scores_review",
              rolled_sets: sets,
              resolved_values: resolved.values,
              suggested_assignment: resolved.assignment,
            },
          },
        });
        const followUp = await sendCharacterBuilderMessage(builderCharacter.id, {
          language: locale,
          message:
            locale === "de"
              ? `Die sieben 4d6-Würfe sind jetzt abgeschlossen. Die Einzelwürfe waren ${sets
                  .map((set, index) => `Wurf ${index + 1}: ${set.join(", ")}`)
                  .join(" | ")}. Der schwächste Gesamtwurf wurde automatisch gestrichen, die sechs verwendeten Werte sind ${resolved.values.join(
                  ", "
                )}. Bitte gib diese Werte jetzt im Chat sauber aus, erkläre kurz, dass der schlechteste Wurf gestrichen wurde, frage, ob alles stimmt, und führe dann direkt durch die Verteilung auf Stärke, Geschicklichkeit, Konstitution, Intelligenz, Weisheit und Charisma weiter.`
              : `The seven 4d6 rolls are complete. The individual rolls were ${sets
                  .map((set, index) => `Roll ${index + 1}: ${set.join(", ")}`)
                  .join(" | ")}. The lowest total was removed automatically, leaving these six values: ${resolved.values.join(
                  ", "
                )}. Present the values clearly, briefly explain that the lowest roll was removed, ask for confirmation, then guide the player through assigning them to Strength, Dexterity, Constitution, Intelligence, Wisdom, and Charisma.`,
        });
        const nextCharacter = followUp.character ?? updatedDraft;
        setBuilderCharacter(nextCharacter);
        upsertRosterCharacter(nextCharacter);
        setBuilderMessages(followUp.messages);
        syncSheetForm(nextCharacter);
        setAssignment(normalizeAssignment(resolved.assignment));
        setResolvedValues(resolved.values);
        setCurrentRollIndex(0);
        setConfirmedRolls(Array.from({ length: guidedRollCount }, () => false));
        notify({ title: tr("Dice Step", "Würfelschritt"), message: tr("All seven rolls were processed and the best six passed to the builder.", "Alle sieben Würfe wurden verarbeitet und die besten sechs an den Builder übergeben."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Dice Step",
          message: error instanceof Error ? error.message : tr("Could not pass the rolled values to the builder.", "Die gewürfelten Werte konnten nicht an den Builder übergeben werden."),
          tone: "error",
        });
      } finally {
        setIsBuilderResponding(false);
      }
    });
  }

  function handleConfirmCurrentRoll() {
    const sets = currentRollSets();
    const current = sets[currentRollIndex];
    if (current.some((value) => value < 1 || value > 6)) {
      setRollCameraMessage(tr("Enter or detect four valid d6 values for the current roll first.", "Bitte zuerst vier gültige d6-Werte für den aktuellen Wurf eingeben oder erkennen lassen."));
      return;
    }
    const nextConfirmed = confirmedRolls.map((entry, index) => (index === currentRollIndex ? true : entry));
    setConfirmedRolls(nextConfirmed);
    const nextIndex = nextConfirmed.findIndex((entry) => !entry);
    if (nextIndex === -1) {
      finalizeRollSequence(sets);
      return;
    }
    setCurrentRollIndex(nextIndex);
    setRollCameraMessage(tr(`Roll ${currentRollIndex + 1} confirmed. Now make roll ${nextIndex + 1} with 4d6.`, `Wurf ${currentRollIndex + 1} bestätigt. Würfle jetzt Wurf ${nextIndex + 1} mit 4d6.`));
  }

  function handleStartBuilder() {
    if (!selectedRulesetGroup || selectedDocumentIds.length === 0) {
      notify({ title: "Builder", message: tr("Select at least one rulebook.", "Bitte mindestens ein Regelbuch auswählen."), tone: "warning" });
      return;
    }
    setBuilderReferencePopup(null);
    startTransition(async () => {
      try {
        const response = await startCharacterBuilder({
          campaign_id: selectedCampaignId || undefined,
          ruleset_work: selectedRulesetGroup.work,
          ruleset_version: selectedRulesetGroup.version,
          selected_document_ids: selectedDocumentIds,
          name: builderCharacterName || undefined,
          player_name: builderPlayerName || undefined,
          language: locale,
        });
        setBuilderCharacter(response.character);
        upsertRosterCharacter(response.character);
        setBuilderMessages(response.messages);
        syncSheetForm(response.character);
        setBuilderStep("chat");
        notify({ title: tr("Character Draft", "Charakterentwurf"), message: tr("A new draft was started.", "Ein neuer Entwurf wurde gestartet."), tone: "success" });
      } catch (error) {
        notify({
          title: "Character Builder",
          message: error instanceof Error ? error.message : tr("Could not start builder.", "Builder konnte nicht gestartet werden."),
          tone: "error",
        });
      }
    });
  }

  function handleSendBuilderMessage() {
    if (!builderCharacter || !builderInput.trim() || isBuilderResponding) {
      return;
    }
    const outgoingMessage = builderInput.trim();
    const optimisticUserMessage: CharacterBuilderMessage = {
      role: "user",
      content: outgoingMessage,
      created_at: new Date().toISOString(),
    };
    setBuilderMessages((current) => [...current, optimisticUserMessage]);
    setBuilderInput("");
    setIsBuilderResponding(true);

    startTransition(async () => {
      try {
        const response = await sendCharacterBuilderMessage(builderCharacter.id, {
          message: outgoingMessage,
          language: locale,
        });
        setBuilderCharacter(response.character);
        upsertRosterCharacter(response.character);
        setBuilderMessages(response.messages);
        syncSheetForm(response.character);
        if (response.ui_action === "open_document" && response.ui_payload) {
          const documentId = String(response.ui_payload.document_id ?? "").trim();
          const documentName = String(response.ui_payload.document_name ?? "").trim();
          const pageValue = Number(response.ui_payload.document_page);
          if (documentId && documentName) {
            setBuilderReferencePopup({
              documentId,
              documentName,
              page: Number.isFinite(pageValue) && pageValue > 0 ? pageValue : null,
            });
          } else {
            setBuilderReferencePopup(null);
          }
        } else {
          setBuilderReferencePopup(null);
        }
        if (safeString(response.character.metadata.creation_method) === "rolled") {
          setAbilityMethod("rolled");
        } else if (safeString(response.character.metadata.creation_method) === "point_buy") {
          setAbilityMethod("point_buy");
        } else if (safeString(response.character.metadata.creation_method) === "standard") {
          setAbilityMethod("standard");
        }
        notify({ title: "Builder", message: tr("The AI updated the draft.", "Die KI hat den Entwurf aktualisiert."), tone: "success" });
      } catch (error) {
        setBuilderMessages((current) => current.filter((message) => message !== optimisticUserMessage));
        setBuilderInput(outgoingMessage);
        notify({
          title: "Builder",
          message: error instanceof Error ? error.message : tr("Could not process message.", "Nachricht konnte nicht verarbeitet werden."),
          tone: "error",
        });
      } finally {
        setIsBuilderResponding(false);
      }
    });
  }

  async function handlePlayBuilderSpeech(message: CharacterBuilderMessage) {
    const speechKey = `${message.created_at}:${message.content}`;
    builderSpeechRequestKeyRef.current = speechKey;
    setBuilderSpeechLoadingKey(speechKey);
    setBuilderSpeechError(null);
    try {
      const response = await fetch(
        `${apiBaseUrl}/api/tts-audio?voice=${encodeURIComponent("narrator-default")}&language=${encodeURIComponent(locale)}&text=${encodeURIComponent(
          buildBuilderSpeechText(message.content)
        )}`,
        { cache: "no-store" }
      );
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `${response.status}`);
      }
      const blob = await response.blob();
      if (builderSpeechObjectUrlRef.current) {
        const currentAudio = builderSpeechAudioRef.current;
        if (currentAudio) {
          currentAudio.pause();
          currentAudio.removeAttribute("src");
          currentAudio.load();
        }
        URL.revokeObjectURL(builderSpeechObjectUrlRef.current);
      }
      const objectUrl = URL.createObjectURL(blob);
      if (builderSpeechRequestKeyRef.current !== speechKey) {
        URL.revokeObjectURL(objectUrl);
        return;
      }
      builderSpeechObjectUrlRef.current = objectUrl;
      setBuilderSpeechUrl(objectUrl);
      setBuilderSpeechActiveKey(speechKey);
      const audio = builderSpeechAudioRef.current;
      if (!audio) {
        throw new Error(tr("Audio element is not ready.", "Audioelement ist nicht bereit."));
      }
      audio.src = objectUrl;
      audio.currentTime = 0;
      audio.load();
      try {
        await audio.play();
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        throw error;
      }
    } catch (error) {
      setBuilderSpeechActiveKey(speechKey);
      setBuilderSpeechUrl("");
      setBuilderSpeechError(error instanceof Error && error.message ? error.message : tr("Could not play speech output.", "Sprachausgabe konnte nicht abgespielt werden."));
    } finally {
      setBuilderSpeechLoadingKey("");
    }
  }

  async function handleToggleBuilderRecording() {
    if (isBuilderRecording) {
      builderRecorderRef.current?.stop();
      return;
    }
    setBuilderSTTError(null);
    setBuilderSTTStatus(tr("Starting microphone...", "Mikrofon wird gestartet …"));
    if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia) {
      setBuilderSTTError(tr("Microphone access requires HTTPS. Open the app over HTTPS or explicitly allow the LAN address in your browser.", "Mikrofonzugriff benötigt HTTPS. Öffne die App über eine sichere HTTPS-Adresse oder erlaube die LAN-Adresse ausdrücklich im Browser."));
      setBuilderSTTStatus("");
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      builderAudioStreamRef.current?.getTracks().forEach((track) => track.stop());
      builderAudioStreamRef.current = stream;
      const mimeType = pickRecordingMimeType();
      const recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream);
      builderRecordedChunksRef.current = [];
      recorder.onstart = () => {
        setIsBuilderRecording(true);
        setBuilderSTTStatus(tr("Speak now. Recording stops automatically after 12 seconds.", "Sprich jetzt. Die Aufnahme endet automatisch nach 12 Sekunden."));
        if (builderStopTimerRef.current) {
          window.clearTimeout(builderStopTimerRef.current);
        }
        builderStopTimerRef.current = window.setTimeout(() => {
          builderStopTimerRef.current = null;
          if (recorder.state !== "inactive") {
            recorder.stop();
          }
        }, BUILDER_MAX_RECORDING_MS);
      };
      recorder.ondataavailable = (event) => {
        if (event.data && event.data.size > 0) {
          builderRecordedChunksRef.current.push(event.data);
        }
      };
      recorder.onerror = () => {
        if (builderStopTimerRef.current) {
          window.clearTimeout(builderStopTimerRef.current);
          builderStopTimerRef.current = null;
        }
        setBuilderSTTError(tr("Could not start voice recording.", "Sprachaufnahme konnte nicht gestartet werden."));
        setBuilderSTTStatus("");
        setIsBuilderRecording(false);
      };
      recorder.onstop = async () => {
        if (builderStopTimerRef.current) {
          window.clearTimeout(builderStopTimerRef.current);
          builderStopTimerRef.current = null;
        }
        setIsBuilderRecording(false);
        setBuilderSTTStatus("");
        const recordedType = recorder.mimeType || builderRecordedChunksRef.current[0]?.type || "audio/webm";
        const extension = recordedType.includes("ogg") ? "ogg" : recordedType.includes("mp4") ? "mp4" : "webm";
        const blob = new Blob(builderRecordedChunksRef.current, { type: recordedType });
        builderRecordedChunksRef.current = [];
        stream.getTracks().forEach((track) => track.stop());
        builderAudioStreamRef.current = null;
        if (blob.size === 0) {
          setBuilderSTTError(tr("No voice recording detected.", "Keine Sprachaufnahme erkannt."));
          return;
        }
        setIsBuilderTranscribing(true);
        setBuilderSTTStatus(tr("Transcribing speech...", "Spracherkennung läuft …"));
        try {
          const transcript = await uploadSTTBlob(blob, `builder.${extension}`, locale);
          if (!transcript) {
            throw new Error(tr("No speech detected.", "Keine Sprache erkannt."));
          }
          setBuilderInput((current) => (current.trim() ? `${current.trim()} ${transcript}` : transcript));
          setBuilderSTTStatus(tr("Transcript applied.", "Transkript übernommen."));
        } catch (error) {
          setBuilderSTTError(error instanceof Error && error.message ? error.message : tr("Speech recognition failed.", "Spracherkennung fehlgeschlagen."));
          setBuilderSTTStatus("");
        } finally {
          setIsBuilderTranscribing(false);
        }
      };
      builderRecorderRef.current = recorder;
      recorder.start();
    } catch (error) {
      setBuilderSTTError(error instanceof Error && error.message ? error.message : tr("Could not open microphone.", "Mikrofon konnte nicht geöffnet werden."));
      setBuilderSTTStatus("");
    }
  }

  useEffect(() => {
    return () => {
      if (builderStopTimerRef.current) {
        window.clearTimeout(builderStopTimerRef.current);
      }
    };
  }, []);

  function handleApplySheetPatch() {
    if (!builderCharacter) {
      return;
    }
    const patch: CharacterBuilderPatch = {
      name: sheetForm.name,
      player_name: sheetForm.player_name,
      class_and_level: sheetForm.class_and_level,
      race: sheetForm.race,
      background: sheetForm.background,
      alignment: sheetForm.alignment,
      languages: splitMetadataList(sheetForm.languages),
      armor_class: parseNumericText(sheetForm.armor_class),
      hit_point_max: parseNumericText(sheetForm.hit_point_max),
      speed: sheetForm.speed,
      proficiency_bonus: sheetForm.proficiency_bonus,
      metadata: {
        inspiration: sheetForm.inspiration,
        experience_points: sheetForm.experience_points,
        hit_dice: sheetForm.hit_dice,
        concept: sheetForm.concept,
        builder_stage: sheetForm.builder_stage,
        personality_traits: sheetForm.personality_traits,
        ideals: sheetForm.ideals,
        bonds: sheetForm.bonds,
        flaws: sheetForm.flaws,
        backstory: sheetForm.backstory,
        age: sheetForm.age,
        size: sheetForm.size,
        weight: sheetForm.weight,
        eyes: sheetForm.eyes,
        skin: sheetForm.skin,
        hair: sheetForm.hair,
        allies: sheetForm.allies,
        senses: sheetForm.senses,
        current_hit_points: sheetForm.current_hit_points,
        temporary_hit_points: sheetForm.temporary_hit_points,
        tools_and_proficiencies: splitMetadataList(sheetForm.tools_and_proficiencies),
        weapon_notes: splitMetadataList(sheetForm.weapon_notes),
        starting_money: sheetForm.starting_money,
        current_money: sheetForm.current_money,
        current_inventory: splitMetadataList(sheetForm.current_inventory),
        level_up_available: sheetForm.level_up_available,
        combat_overview: sheetForm.combat_overview,
        combat_attacks: sheetForm.combat_attacks,
        skill_proficiencies: splitMetadataList(sheetForm.skill_proficiencies),
        saving_throw_proficiencies: splitMetadataList(sheetForm.saving_throw_proficiencies),
        starting_equipment: splitMetadataList(sheetForm.starting_equipment),
        spells: splitMetadataList(sheetForm.spells),
        spell_save_dc: sheetForm.spell_save_dc,
        spell_attack_bonus: sheetForm.spell_attack_bonus,
        spell_attacks: sheetForm.spell_attacks,
        spell_notes: sheetForm.spell_notes,
      },
    };
    startTransition(async () => {
      try {
        const updated = await applyCharacterBuilderPatch(builderCharacter.id, { patch });
        setBuilderCharacter(updated);
        upsertRosterCharacter(updated);
        syncSheetForm(updated);
        setIsSheetEditMode(false);
        notify({ title: tr("Character Sheet", "Charakterbogen"), message: tr("Sheet changes were saved.", "Änderungen am Bogen wurden gespeichert."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Character Sheet",
          message: error instanceof Error ? error.message : tr("Could not save sheet.", "Bogen konnte nicht gespeichert werden."),
          tone: "error",
        });
      }
    });
  }

  function handleResolveAbilities() {
    const payload =
      abilityMethod === "rolled"
        ? {
            method: "rolled" as const,
            class: sheetForm.class_and_level,
            rolled_sets: parseRollSets(rolledSetsText),
          }
        : abilityMethod === "point_buy"
          ? {
              method: "point_buy" as const,
              class: sheetForm.class_and_level,
              point_buy: [15, 14, 13, 12, 10, 8],
            }
          : {
              method: "standard" as const,
              class: sheetForm.class_and_level,
            };

    startTransition(async () => {
      try {
        const response = await resolveAbilityScores(payload);
        setResolvedValues(response.values);
        setAssignment(normalizeAssignment(response.assignment));
        setRuleSummary(response.rule_summary);
        setAssignmentConfirmed(false);
        setValidationMessage("");
      } catch (error) {
        notify({
          title: "Ability Scores",
          message: error instanceof Error ? error.message : tr("Could not resolve values.", "Werte konnten nicht aufgelöst werden."),
          tone: "error",
        });
      }
    });
  }

  function handleValidateAbilities() {
    startTransition(async () => {
      try {
        const response = await validateAbilityAssignment({
          class: sheetForm.class_and_level,
          values: resolvedValues,
          assignment,
        });
        setAssignmentConfirmed(response.valid);
        setValidationMessage(
          response.valid
            ? tr("Assignment confirmed. The values can now be applied to the draft.", "Zuordnung bestätigt. Die Werte können jetzt in den Entwurf übernommen werden.")
            : `Zuordnung muss korrigiert werden. Missing: ${response.missing_abilities.join(", ") || "none"}`
        );
      } catch (error) {
        notify({
          title: "Ability Validation",
          message: error instanceof Error ? error.message : tr("Could not validate assignment.", "Zuordnung konnte nicht geprüft werden."),
          tone: "error",
        });
      }
    });
  }

  function handleApplyAbilities() {
    if (!builderCharacter || !assignmentConfirmed) {
      notify({ title: tr("Ability Scores", "Attributwerte"), message: tr("Confirm the assignment first.", "Bestätige zuerst die Zuordnung."), tone: "warning" });
      return;
    }
    startTransition(async () => {
      try {
        const updated = await applyCharacterBuilderPatch(builderCharacter.id, {
          patch: {
            abilities: assignment,
            metadata: {
              creation_method: abilityMethod,
              resolved_values: resolvedValues,
            },
          },
        });
        upsertRosterCharacter(updated);
        const followUp = await sendCharacterBuilderMessage(builderCharacter.id, {
          language: locale,
          message:
            locale === "de"
              ? `Die bestätigten Attributwürfe sind ${resolvedValues.join(", ")}. Die Zuordnung lautet STR ${assignment.strength}, DEX ${assignment.dexterity}, CON ${assignment.constitution}, INT ${assignment.intelligence}, WIS ${assignment.wisdom}, CHA ${assignment.charisma}. Bitte prüfe die Werte, frage kurz, ob alles stimmt, und fahre dann mit dem nächsten Schritt der Charaktererstellung fort.`
              : `The confirmed ability rolls are ${resolvedValues.join(", ")}. The assignment is STR ${assignment.strength}, DEX ${assignment.dexterity}, CON ${assignment.constitution}, INT ${assignment.intelligence}, WIS ${assignment.wisdom}, CHA ${assignment.charisma}. Verify the values, briefly ask whether everything is correct, then continue with the next character creation step.`,
        });
        setBuilderCharacter(followUp.character);
        upsertRosterCharacter(followUp.character);
        setBuilderMessages(followUp.messages);
        syncSheetForm(followUp.character);
        setIsRollModalOpen(false);
        notify({ title: tr("Ability Scores", "Attributwerte"), message: tr("Values were applied to the draft.", "Werte wurden in den Entwurf übernommen."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Ability Scores",
          message: error instanceof Error ? error.message : tr("Could not save values.", "Werte konnten nicht gespeichert werden."),
          tone: "error",
        });
      }
    });
  }

  function handleFinishBuilder() {
    if (!builderCharacter) {
      return;
    }
    startTransition(async () => {
      try {
        const updated = await finishCharacterBuilder(builderCharacter.id);
        setBuilderCharacter(updated);
        upsertRosterCharacter(updated);
        if (initialBuilderSeed?.playerSlotId) {
          await updatePlayerSlotCharacter(initialBuilderSeed.playerSlotId, { character_id: updated.id });
        }
        notify({ title: tr("Character Ready", "Charakter bereit"), message: tr("The character is now marked as ready.", "Der Charakter ist jetzt als bereit markiert."), tone: "success" });
        if (initialBuilderSeed?.returnPath) {
          router.push(initialBuilderSeed.returnPath);
          return;
        }
        router.refresh();
        closeBuilder();
      } catch (error) {
        notify({
          title: tr("Character Ready", "Charakter bereit"),
          message: error instanceof Error ? error.message : tr("Could not finish builder.", "Builder konnte nicht abgeschlossen werden."),
          tone: "error",
        });
      }
    });
  }

  function handleDuplicateCharacter(character: RosterCharacter) {
    startTransition(async () => {
      try {
        const created = await createCharacter({
          campaign_id: character.campaign_id,
          name: `${character.name} Copy`,
          player_name: character.player_name,
          class_and_level: character.class_and_level,
          background: character.background,
          race: character.race,
          alignment: character.alignment,
          armor_class: character.armor_class,
          speed: character.speed,
          hit_point_max: character.hit_point_max,
          proficiency_bonus: character.proficiency_bonus,
          abilities: character.abilities,
          languages: character.languages,
          features: character.features,
          metadata: {
            ...character.metadata,
            builder_status: "draft",
          },
        });
        upsertRosterCharacter(created);
        notify({ title: tr("Character Copy", "Charakterkopie"), message: tr("Copy created.", "Kopie wurde erstellt."), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Character Copy",
          message: error instanceof Error ? error.message : tr("Could not create copy.", "Kopie konnte nicht erstellt werden."),
          tone: "error",
        });
      }
    });
  }

  function handleDeleteCharacter(character: RosterCharacter) {
    if (!window.confirm(tr(`Really delete character "${character.name}"?`, `Charakter "${character.name}" wirklich löschen?`))) {
      return;
    }
    startTransition(async () => {
      try {
        await apiDelete<{ deleted: boolean }>(`/api/characters/${character.id}`);
        removeRosterCharacter(character.id);
        notify({ title: tr("Character Deleted", "Charakter gelöscht"), message: tr(`${character.name} was deleted.`, `${character.name} wurde gelöscht.`), tone: "success" });
        router.refresh();
      } catch (error) {
        notify({
          title: "Character Deleted",
          message: error instanceof Error ? error.message : tr("Could not delete character.", "Charakter konnte nicht gelöscht werden."),
          tone: "error",
        });
      }
    });
  }

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow={tr("Characters", "Charaktere")}
        title={tr("Character roster with AI-guided creation", "Charakterübersicht mit KI-geführter Erstellung")}
        description={tr("Create characters from your rulesets and develop them in an AI-guided, continuously saved draft.", "Erstelle Charaktere aus deinen Regelwerken und entwickle sie in einem KI-geführten, laufend gespeicherten Entwurf.")}
      />

      <div className="dashboard-grid">
        <Panel title={tr("Roster Overview", "Charakterübersicht")} description={tr("View all characters and open the builder when needed.", "Alle Charaktere anzeigen und bei Bedarf den Builder öffnen.")} className="hero-panel">
          <div className="stat-grid">
            <StatCard label={tr("Characters", "Charaktere")} value={roster.length} />
            <StatCard label={tr("Drafts", "Entwürfe")} value={roster.filter((character) => character.statusLabel === "draft").length} />
            <StatCard label={tr("Ready", "Bereit")} value={roster.filter((character) => character.statusLabel === "ready").length} />
            <StatCard label={tr("Rulesets", "Regelwerke")} value={rulesetGroups.length} />
          </div>
          <div className="button-row">
            <button className="studio-button studio-button--primary" onClick={openNewBuilder} type="button">
              <Plus size={16} />
              {tr("Create New Character", "Neuen Charakter erstellen")}
            </button>
          </div>
        </Panel>
      </div>

      <Panel title={tr("Character Roster", "Charakterliste")} description={tr("Filter, open, edit, duplicate, or delete characters.", "Charaktere filtern, öffnen, bearbeiten, duplizieren oder löschen.")}>
        <div className="library-toolbar">
          <div className="library-toolbar__group">
            <span className="library-toolbar__label">{tr("Ruleset", "Regelwerk")}</span>
            <div className="meta-chip-row">
              {availableRulesetFilters.map((option) => (
                <button
                  className={`filter-chip${rulesetFilter === option ? " is-active" : ""}`}
                  key={option}
                  onClick={() => setRulesetFilter(option)}
                  type="button"
                >
                  {option === "all" ? tr("All", "Alle") : option}
                </button>
              ))}
            </div>
          </div>
          <div className="library-toolbar__group">
            <span className="library-toolbar__label">{tr("Status", "Status")}</span>
            <div className="meta-chip-row">
              {["all", "draft", "ready", "assigned"].map((option) => (
                <button
                  className={`filter-chip${statusFilter === option ? " is-active" : ""}`}
                  key={option}
                  onClick={() => setStatusFilter(option)}
                  type="button"
                >
                  {option === "all" ? tr("All", "Alle") : option}
                  </button>
              ))}
            </div>
          </div>
        </div>

        <div className="card-grid card-grid--three">
          {filteredCharacters.length === 0 ? <p className="empty-copy">{tr("No characters match the current filter.", "Keine Charaktere entsprechen dem aktuellen Filter.")}</p> : null}
          {filteredCharacters.map((character) => (
            <article className="media-card media-card--roster" key={character.id}>
              <div className="media-card__cover media-card__cover--character">
                <User size={34} />
              </div>
              <div className="media-card__body">
                <div className="media-card__head">
                  <h3>{character.name}</h3>
                  <div className="meta-chip-row">
                    <StatusPill tone="info">{character.rulesetLabel}</StatusPill>
                    <StatusPill tone={character.statusLabel === "draft" ? "warning" : character.statusLabel === "assigned" ? "ready" : "default"}>
                      {statusLabel(character.statusLabel)}
                    </StatusPill>
                  </div>
                </div>
                <p>{character.race || tr("Unknown ancestry", "Unbekannte Abstammung")} · {character.class_and_level || tr("Role not chosen yet", "Rolle noch nicht gewählt")}</p>
                <div className="character-roster-abilities">
                  {abilityOrder.map((ability) => (
                    <div className="character-roster-ability" key={`${character.id}-${ability}`}>
                      <span>{(locale === "de" ? abilityLabels[ability] : englishAbilityLabels[ability]).slice(0, 3)}</span>
                      <strong>{character.abilities[ability] || "—"}</strong>
                    </div>
                  ))}
                </div>
                {character.concept ? <p className="muted-copy">{character.concept}</p> : <p className="muted-copy">{tr("No concept saved yet.", "Noch kein Konzept gespeichert.")}</p>}
                {character.selectedDocumentNames.length > 0 ? (
                  <div className="meta-chip-row">
                    {character.selectedDocumentNames.slice(0, 3).map((name) => (
                      <StatusPill key={name} tone="default">{name}</StatusPill>
                    ))}
                  </div>
                ) : null}
                <div className="button-row">
                  <button
                    className="studio-button studio-button--ghost studio-button--inline"
                    onClick={() => openBuilderForCharacter(character)}
                    type="button"
                  >
                    <PenSquare size={14} />
                    {tr("Edit", "Bearbeiten")}
                  </button>
                  <button
                    className="studio-button studio-button--ghost studio-button--inline"
                    onClick={() => handleDuplicateCharacter(character)}
                    type="button"
                  >
                    <Copy size={14} />
                    {tr("Duplicate", "Duplizieren")}
                  </button>
                  <button
                    className="studio-button studio-button--danger studio-button--inline"
                    onClick={() => handleDeleteCharacter(character)}
                    type="button"
                  >
                    <Trash2 size={14} />
                    {tr("Delete", "Löschen")}
                  </button>
                </div>
              </div>
            </article>
          ))}
        </div>
      </Panel>

      {isBuilderOpen ? (
        <div className="modal-overlay" onClick={closeBuilder} role="presentation">
          <section
            aria-modal="true"
            className="modal-card modal-card--builder"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">{tr("Character Builder", "Charakter-Builder")}</p>
                <h2>{builderStep === "start" ? tr("Prepare New Character", "Neuen Charakter vorbereiten") : tr("AI-guided Character Draft", "KI-geführter Charakterentwurf")}</h2>
                <p className="muted-copy">
                  {builderStep === "start"
                    ? tr("First select a ruleset and source books. Then chat with the AI while the character sheet develops alongside it.", "Wähle zuerst Regelwerk und Buchbasis. Danach startet der KI-Dialog, während der Charakterbogen mitwächst.")
                    : tr("Chat with the builder while the character sheet is saved live. Every confirmed decision updates the draft.", "Sprich mit dem Builder, während der Charakterbogen live gespeichert wird. Jede bestätigte Entscheidung aktualisiert den Entwurf.")}
                </p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={closeBuilder} type="button">
                {tr("Close", "Schließen")}
              </button>
            </div>

            {builderStep === "start" ? (
              <div className="page-stack">
                {initialBuilderSeed ? (
                  <div className="story-box story-box--hero">
                    <User size={16} />
                    <div>
                      <strong>{tr("Portal defaults active", "Portal-Vorbelegung aktiv")}</strong>
                      <p>
                        {initialBuilderSeed.playerName} · {initialBuilderSeed.rulesetWork} {initialBuilderSeed.rulesetVersion}
                      </p>
                    </div>
                  </div>
                ) : null}
                <div className="dual-field-grid">
                  <label className="field-stack">
                    <span>{tr("Ruleset", "Regelwerk")}</span>
                    <select onChange={(event) => setSelectedRulesetKey(event.target.value)} value={selectedRulesetKey}>
                      {rulesetGroups.map((group) => (
                        <option key={group.key} value={group.key}>
                          {group.work} {group.version}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="field-stack">
                    <span>{tr("Campaign", "Kampagne")}</span>
                    <select onChange={(event) => setSelectedCampaignId(event.target.value)} value={selectedCampaignId}>
                      <option value="">{tr("Do not assign a campaign yet", "Noch keiner Kampagne zuordnen")}</option>
                      {campaigns.map((campaign) => (
                        <option key={campaign.id} value={campaign.id}>
                          {campaign.name}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>

                <div className="dual-field-grid">
                  <label className="field-stack">
                    <span>{tr("Character name", "Charaktername")}</span>
                    <input onChange={(event) => setBuilderCharacterName(event.target.value)} placeholder={tr("Optional: character name", "Optional: Charaktername")} value={builderCharacterName} />
                  </label>
                  <label className="field-stack">
                    <span>{tr("Player name", "Spielername")}</span>
                    <input onChange={(event) => setBuilderPlayerName(event.target.value)} placeholder={tr("Optional: player name", "Optional: Spielername")} value={builderPlayerName} />
                  </label>
                </div>

                <Panel title={tr("Source Books", "Buchbasis")} description={tr("The AI prioritizes these books. Unselected books are not used as default sources.", "Die KI priorisiert diese Bücher. Nicht ausgewählte Bücher werden nicht als Standardquelle verwendet.")}>
                  <div className="meta-chip-row">
                    {selectedRulesetGroup?.work === "5E" && selectedRulesetGroup?.version === "2014" ? (
                      <>
                        <StatusPill tone="ready">{tr("Built-in example", "Integriertes Beispiel")}</StatusPill>
                        <StatusPill tone="info">{tr("Character Builder Guide", "Leitfaden zur Charaktererstellung")}</StatusPill>
                        <StatusPill tone="default">{tr("Level-Up Guide", "Leitfaden zum Stufenaufstieg")}</StatusPill>
                      </>
                    ) : (
                      <StatusPill tone="default">{tr("No built-in example guide for this ruleset", "Kein integrierter Beispiel-Guide für dieses Regelwerk")}</StatusPill>
                    )}
                  </div>
                  <div className="builder-documents-grid">
                    {selectedRulesetGroup?.documents.map((document) => {
                      const checked = selectedDocumentIds.includes(document.id);
                      return (
                        <label className={`builder-document-card${checked ? " is-selected" : ""}`} key={document.id}>
                          <input
                            checked={checked}
                            onChange={() => {
                              setSelectedDocumentIds((current) =>
                                checked ? current.filter((id) => id !== document.id) : [...current, document.id]
                              );
                            }}
                            type="checkbox"
                          />
                          <div>
                            <strong>{document.name}</strong>
                            <p>{document.chunk_count} {tr("chunks indexed", "Abschnitte indiziert")}</p>
                          </div>
                        </label>
                      );
                    })}
                  </div>
                </Panel>
              </div>
            ) : (
              <div className="character-builder-layout">
                <section className="character-builder-chat">
                  <div className="character-builder-chat__head">
                    <div>
                      <strong>{tr("Builder Chat", "Builder-Chat")}</strong>
                      <p>
                        {tr("Text and voice in the builder.", "Text und Sprache im Builder.")}
						{safeString(builderCharacter?.metadata.ruleset_work) === "5E" &&
						  safeString(builderCharacter?.metadata.ruleset_version) === "2014"
						  ? " The bundled 5E-compatible SRD 5.1 example guide is active."
                          : ""}
                      </p>
                    </div>
                    <Bot size={18} />
                  </div>
                  <div className="character-builder-transcript">
                    {builderMessages.map((message, index) => {
                      const speechKey = `${message.created_at}:${message.content}`;
                      return (
                        <article className={`builder-message builder-message--${message.role}`} key={`${message.created_at}-${index}`}>
                          <strong>{message.role === "assistant" ? tr("AI DM", "KI-Spielleiter") : tr("You", "Du")}</strong>
                          <p>{message.content}</p>
                          {message.role === "assistant" ? (
                            <div className="builder-message__voice">
                              <div className="builder-message__voice-head">
                                <span className="builder-message__voice-label">
                                  <Volume2 size={14} />
                                  {tr("Friendly narrator voice", "Freundliche Erzählerstimme")}
                                </span>
                                <button
                                  className="studio-button studio-button--ghost studio-button--inline"
                                  disabled={builderSpeechLoadingKey === speechKey}
                                  onClick={() => handlePlayBuilderSpeech(message)}
                                  type="button"
                                >
                                  {builderSpeechLoadingKey === speechKey ? tr("Loading...", "Lädt …") : tr("Play", "Abspielen")}
                                </button>
                              </div>
                              <audio
                                controls={builderSpeechActiveKey === speechKey && Boolean(builderSpeechUrl)}
                                ref={builderSpeechAudioRef}
                              >
                                {tr("Your browser cannot play the character builder voice.", "Dein Browser kann die Charakter-Builder-Stimme nicht abspielen.")}
                              </audio>
							  <p className="player-audio-note">{tr("AI-generated voice", "KI-generierte Stimme")}</p>
                              {builderSpeechActiveKey === speechKey && builderSpeechUrl ? (
                                null
                              ) : null}
                              {builderSpeechActiveKey === speechKey && builderSpeechError ? (
                                <p className="player-audio-note">{builderSpeechError}</p>
                              ) : null}
                            </div>
                          ) : null}
                        </article>
                      );
                    })}
                    {isBuilderResponding ? (
                      <article className="builder-message builder-message--assistant builder-message--working">
                        <strong>{tr("AI DM", "KI-Spielleiter")}</strong>
                        <div className="builder-thinking">
                          <span className="builder-thinking__dot" />
                          <span className="builder-thinking__dot" />
                          <span className="builder-thinking__dot" />
                          <p>{tr("The AI is working on your character draft.", "Die KI arbeitet gerade an deinem Charakterentwurf.")}</p>
                        </div>
                      </article>
                    ) : null}
                    <div ref={transcriptEndRef} />
                  </div>
                  <div className="character-builder-composer">
                    <textarea
                      onChange={(event) => setBuilderInput(event.target.value)}
                      placeholder={tr("Describe your concept, role, origin, or answer the AI's latest question.", "Beschreibe Konzept, Rolle oder Herkunft oder beantworte die letzte Frage der KI.")}
                      value={builderInput}
                    />
                    {builderSTTStatus ? <p className="player-audio-note">{builderSTTStatus}</p> : null}
                    {builderSTTError ? <p className="player-audio-note">{builderSTTError}</p> : null}
                    <div className="button-row">
                      <button
                        className="studio-button studio-button--ghost"
                        disabled={isPending || isBuilderResponding || isBuilderTranscribing}
                        onClick={handleToggleBuilderRecording}
                        type="button"
                      >
                        {isBuilderRecording ? <MicOff size={16} /> : <Mic size={16} />}
                        {isBuilderRecording ? tr("Stop recording", "Aufnahme beenden") : isBuilderTranscribing ? tr("Transcribing...", "Wird transkribiert …") : tr("Voice input", "Spracheingabe")}
                      </button>
                      <button
                        className="studio-button studio-button--primary"
                        disabled={isPending || isBuilderResponding || !builderInput.trim()}
                        onClick={handleSendBuilderMessage}
                        type="button"
                      >
                        <MessageSquare size={16} />
                        {isBuilderResponding ? tr("AI is responding...", "KI antwortet …") : tr("Send message", "Nachricht senden")}
                      </button>
                    </div>
                  </div>
                </section>

                <section className="character-builder-sheet">
                  <div className="character-builder-sheet__section">
                    <div className="character-builder-sheet__head">
                      <div>
                        <strong>{tr("Live Character Sheet", "Live-Charakterbogen")}</strong>
                        <p>{tr("The AI fills in the sheet with you, one step at a time.", "Die KI füllt den Bogen Schritt für Schritt gemeinsam mit dir aus.")}</p>
                      </div>
                      {builderCharacter ? (
                        <StatusPill tone={safeString(builderCharacter.metadata.builder_status) === "draft" ? "warning" : "ready"}>
                          {statusLabel(safeString(builderCharacter.metadata.builder_status) || "draft")}
                        </StatusPill>
                      ) : null}
                    </div>
                    <CharacterSheetCanvas
                      activeTab={activeSheetTab}
                      assignment={assignment}
                      builderSkillKeys={builderSkillKeys}
                      isEditMode={isSheetEditMode}
                      onFieldChange={(field, value) => setSheetForm((current) => ({ ...current, [field]: value }))}
                      onSave={handleApplySheetPatch}
                      onToggleEditMode={() => setIsSheetEditMode((current) => !current)}
                      onTabChange={setActiveSheetTab}
                      saveDisabled={isPending || !builderCharacter}
                      sheetForm={sheetForm}
                    />
                  </div>

                  <div className="character-builder-sheet__section">
                    <strong>{tr("Source Books", "Buchbasis")}</strong>
                    <div className="meta-chip-row">
                      {(builderCharacter ? splitMetadataList(builderCharacter.metadata.selected_document_names) : []).map((name) => (
                        <StatusPill key={name} tone="default">{name}</StatusPill>
                      ))}
                    </div>
                  </div>
                </section>
              </div>
            )}

            <div className="modal-card__footer">
              <span className="modal-card__spacer">{isPending ? tr("Saving...", "Wird gespeichert …") : ""}</span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={closeBuilder} type="button">
                  {tr("Close", "Schließen")}
                </button>
                {builderStep === "start" ? (
                  <button className="studio-button studio-button--primary" disabled={isPending || selectedDocumentIds.length === 0} onClick={handleStartBuilder} type="button">
                    <Plus size={16} />
                    {tr("Start Builder", "Builder starten")}
                  </button>
                ) : (
                  <button className="studio-button studio-button--primary" disabled={isPending || !builderCharacter} onClick={handleFinishBuilder} type="button">
                    <Check size={16} />
                    {tr("Mark as Ready", "Als bereit markieren")}
                  </button>
                )}
              </div>
            </div>
          </section>
        </div>
      ) : null}

      {builderReferencePopup && builderReferenceDocument ? (
        <div className="player-popup">
          <div className="player-popup__backdrop" onClick={() => setBuilderReferencePopup(null)} />
          <div className="player-popup__panel player-popup__panel--meta">
            <div className="player-popup__header">
              <strong>{builderReferencePopup.documentName}</strong>
              <button className="studio-button studio-button--ghost" onClick={() => setBuilderReferencePopup(null)} type="button">
                {tr("Close", "Schließen")}
              </button>
            </div>
            <div className="player-popup__document">
              <p>
                {builderReferencePopup.page
                  ? tr(`Opened on page ${builderReferencePopup.page}.`, `Auf Seite ${builderReferencePopup.page} geöffnet.`)
                  : tr("The rulebook is open.", "Das Regelwerk ist geöffnet.")}
              </p>
              <div className="player-popup__embed-wrap">
                <iframe className="player-popup__embed" src={builderReferenceFileUrl} title={builderReferencePopup.documentName} />
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {isBuilderOpen && isRollModalOpen ? (
        <div className="modal-overlay" onClick={() => setIsRollModalOpen(false)} role="presentation">
          <section
            aria-modal="true"
            className="modal-card modal-card--roll"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">{tr("Dice Step", "Würfelschritt")}</p>
                <h2>{tr("Roll 4d6 seven times", "Jetzt siebenmal 4d6 würfeln")}</h2>
                <p className="muted-copy">
                  {tr("The AI requested ability rolls. Roll ", "Die KI hat Attributswürfe angefordert. Würfle ")}<strong>{tr("4d6 seven times", "siebenmal 4d6")}</strong>{tr(", use the camera to evaluate and correct each roll, then confirm it. The lowest total is removed and the best six values return to the AI.", ", werte jeden Wurf per Kamera aus, korrigiere ihn bei Bedarf und bestätige ihn. Der niedrigste Gesamtwurf wird entfernt und die besten sechs Werte gehen zurück an die KI.")}
                </p>
              </div>
              <button className="studio-button studio-button--ghost studio-button--inline" onClick={() => setIsRollModalOpen(false)} type="button">
                {tr("Close", "Schließen")}
              </button>
            </div>

            <div className="roll-modal-layout">
              <section className="roll-camera-panel">
                <div className="character-builder-sheet__head">
                  <div>
                    <strong>{tr("Current roll", "Aktueller Wurf")} {currentRollIndex + 1}</strong>
                    <p>{tr("Place only the four d6 for this roll in the camera view.", "Lege nur die vier d6 für diesen Wurf in das Kamerabild.")}</p>
                  </div>
                  <StatusPill tone={rollCameraStatus === "ready" ? "ready" : rollCameraStatus === "error" ? "warning" : "info"}>
                    {statusLabel(rollCameraStatus)}
                  </StatusPill>
                </div>
                <div className="meta-chip-row">
                  <StatusPill tone="info">{confirmedRolls.filter(Boolean).length} / {guidedRollCount} {tr("confirmed", "bestätigt")}</StatusPill>
                  <StatusPill tone="default">{tr("Step", "Schritt")} {currentRollIndex + 1} {tr("of", "von")} {guidedRollCount}</StatusPill>
                </div>
                <div className="camera-preview-shell camera-preview-shell--roll">
                  <video autoPlay className="camera-preview" muted playsInline ref={rollVideoRef} />
                  <canvas hidden ref={rollCanvasRef} />
                  {rollCameraStatus !== "ready" ? (
                    <div className="camera-overlay camera-overlay--idle">
                      <span>{tr("No live preview yet", "Noch keine Live-Vorschau")}</span>
                    </div>
                  ) : null}
                </div>
                <p className="muted-copy">{rollCameraMessage}</p>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void handleStartRollCamera()} type="button">
                    {tr("Start Camera", "Kamera starten")}
                  </button>
                  <button className="studio-button studio-button--primary" disabled={rollCameraStatus !== "ready" || isRollCapturing} onClick={() => void handleDetectCurrentRoll()} type="button">
                    {isRollCapturing ? tr("Evaluating...", "Auswertung läuft …") : tr("Evaluate Camera", "Kamera auswerten")}
                  </button>
                </div>
              </section>

              <section className="roll-entry-panel">
                <div className="roll-step-grid">
                  {currentRollSets().map((set, rowIndex) => (
                    <article className={`roll-step-card${rowIndex === currentRollIndex ? " is-active" : ""}`} key={`roll-set-${rowIndex}`}>
                      <div className="character-builder-sheet__head">
                        <strong>{tr("Roll", "Wurf")} {rowIndex + 1}</strong>
                        {confirmedRolls[rowIndex] ? <StatusPill tone="ready">{tr("confirmed", "bestätigt")}</StatusPill> : null}
                      </div>
                      <div className="roll-dice-row">
                        {set.map((value, dieIndex) => (
                          <label className="roll-die-input" key={`roll-${rowIndex}-${dieIndex}`}>
                            <span>d6 #{dieIndex + 1}</span>
                            <input
                              max={6}
                              min={1}
                              onChange={(event) =>
                                handleRollSetChange(rowIndex, dieIndex, Math.max(1, Math.min(6, Number(event.target.value) || 1)))
                              }
                              type="number"
                              value={value || ""}
                            />
                          </label>
                        ))}
                      </div>
                      {rowIndex === currentRollIndex ? (
                        <div className="button-row">
                          <button className="studio-button studio-button--ghost" onClick={handleConfirmCurrentRoll} type="button">
                            {tr("Confirm This Roll", "Diesen Wurf bestätigen")}
                          </button>
                        </div>
                      ) : null}
                    </article>
                  ))}
                </div>
              </section>
            </div>

            <div className="modal-card__footer">
              <span className="modal-card__spacer">
                {tr("After each confirmation, the next roll opens automatically. After roll 7, the best six values are kept and the lowest is removed.", "Nach jeder Bestätigung öffnet sich automatisch der nächste Wurf. Nach Wurf 7 werden die besten sechs Werte übernommen und der niedrigste entfernt.")}
              </span>
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={() => setIsRollModalOpen(false)} type="button">
                  {tr("Back to Builder", "Zurück zum Builder")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
