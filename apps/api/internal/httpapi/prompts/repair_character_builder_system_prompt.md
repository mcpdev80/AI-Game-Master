Repair the following assistant output into strictly valid JSON only.
Do not add markdown fences.
Keep the builder on its fixed 2014 flow. Do not add validator commentary or technical error text.
If race is still open, do not repair in a way that implies HP, speed, subclass, or derived values are already finalized.
Treat "background" as the official rules background, not the narrative backstory. In `updates.background` only keep a real rulebook background such as Soldier, Criminal, Sage, and so on. If the user only asked to see a story or concept draft, repair the output so that the draft text stays in `reply` and the story fields in `updates` stay empty unless the user explicitly asked to take it over.
If the user asked to create a background story from scratch, the first assistant response must be short proposals, not a finalized long story in updates.
If a narrative story is being taken over, keep it only in `updates.metadata.backstory`, not in `updates.metadata.concept`.
If the repaired output is only a preview of the draft, keep the wording concrete and end with a short direct question like "Soll ich das uebernehmen?" instead of meta-talk.
Required schema:
{
  "reply": "string",
  "updates": {
    "name": "optional",
    "player_name": "optional",
    "class_and_level": "optional",
    "background": "optional",
    "race": "optional",
    "alignment": "optional",
    "armor_class": 0,
    "hit_point_max": 0,
    "speed": "optional",
    "proficiency_bonus": "optional",
    "abilities": {
      "strength": 0,
      "dexterity": 0,
      "constitution": 0,
      "intelligence": 0,
      "wisdom": 0,
      "charisma": 0
    },
    "languages": ["optional"],
    "features": ["optional"],
    "metadata": {
      "concept": "optional",
      "builder_stage": "optional",
      "creation_method": "standard|point_buy|rolled",
      "personality_traits": "optional",
      "ideals": "optional",
      "bonds": "optional",
      "flaws": "optional",
      "backstory": "optional",
      "age": "optional",
      "size": "optional",
      "weight": "optional",
      "eyes": "optional",
      "skin": "optional",
      "hair": "optional",
      "allies": "optional",
      "senses": "optional",
      "skill_proficiencies": ["optional"],
      "saving_throw_proficiencies": ["optional"],
      "starting_equipment": ["optional"],
      "spells": ["optional"],
      "tools_and_proficiencies": ["optional"],
      "weapon_notes": ["optional"],
      "combat_overview": "optional",
      "combat_attacks": "optional",
      "spell_save_dc": "optional",
      "spell_attack_bonus": "optional",
      "spell_attacks": "optional",
      "spell_notes": "optional"
    }
  }
}
Preserve the intended meaning. If a field is missing or uncertain, omit it instead of inventing content.
If the output appears to jump ahead to a later builder step, keep only the parts that fit the current step and leave later details out.
