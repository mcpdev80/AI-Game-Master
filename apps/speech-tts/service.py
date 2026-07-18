import io
import os
import re
import threading
from functools import lru_cache
from importlib.resources import files
from pathlib import Path

import numpy as np
import soundfile as sf
from cached_path import cached_path
from fastapi import FastAPI, HTTPException
from fastapi.responses import Response
from hydra.utils import get_class
from omegaconf import OmegaConf
from pydantic import BaseModel

from f5_tts.infer.utils_infer import infer_process, load_model, load_vocoder, preprocess_ref_audio_text


app = FastAPI(title="AI Game Master F5 TTS")

MODELS_DIR = Path(os.getenv("HF_HOME", "/models")) / "f5"
DEFAULT_VOICE = os.getenv("F5_DEFAULT_VOICE", "builder-friendly-male")
MODEL_REPO = os.getenv("F5_MODEL_REPO", "aihpi/F5-TTS-German")
MODEL_NAME = os.getenv("F5_MODEL_NAME", "F5TTS_Base")
MODEL_CKPT = os.getenv("F5_MODEL_CKPT", "F5TTS_Base/model_365000.safetensors")
MODEL_VOCAB = os.getenv("F5_MODEL_VOCAB", "vocab.txt")
VOCODER_NAME = os.getenv("F5_VOCODER_NAME", "vocos")
DEVICE = os.getenv("F5_DEVICE", "cuda")

DEFAULT_REF_AUDIO = Path(os.getenv("F5_REF_AUDIO", "/app/voices/narrator_male_de.wav"))
DEFAULT_REF_TEXT = Path(os.getenv("F5_REF_TEXT", "/app/voices/narrator_male_de.txt"))

VOICE_PROFILES = {
    "builder-friendly-male": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "narrator-default": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "rules-neutral": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "default-npc": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "orc": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "elf": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "dwarf": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
    "creature": {"ref_audio": DEFAULT_REF_AUDIO, "ref_text": DEFAULT_REF_TEXT},
}

INFERENCE_LOCK = threading.Lock()
MAX_SEGMENT_CHARS = 90


class SpeechRequest(BaseModel):
    input: str
    voice: str | None = None
    response_format: str | None = "wav"
    model: str | None = None
    instruct: str | None = None


def _normalize_text(text: str) -> str:
    cleaned = re.sub(r"\s+", " ", text.replace("\n", " ")).strip()
    if not cleaned:
        return ""
    if cleaned[-1] not in ".!?":
        cleaned = f"{cleaned}."
    return cleaned


def _segment_text(text: str) -> list[str]:
    normalized = _normalize_text(text)
    if not normalized:
        return []

    parts = [chunk.strip() for chunk in re.split(r"(?<=[.!?])\s+", normalized) if chunk.strip()]
    if not parts:
        return [normalized]

    segments: list[str] = []
    current = ""
    for part in parts:
        candidate = part if not current else f"{current} {part}"
        if len(candidate) <= MAX_SEGMENT_CHARS:
            current = candidate
            continue
        if current:
            segments.append(current)
            current = ""
        if len(part) <= MAX_SEGMENT_CHARS:
            current = part
            continue
        words = part.split(" ")
        chunk = ""
        for word in words:
            probe = word if not chunk else f"{chunk} {word}"
            if len(probe) <= MAX_SEGMENT_CHARS:
                chunk = probe
            else:
                if chunk:
                    if chunk[-1] not in ".!?":
                        chunk = f"{chunk}."
                    segments.append(chunk)
                chunk = word
        if chunk:
            if chunk[-1] not in ".!?":
                chunk = f"{chunk}."
            current = chunk
    if current:
        segments.append(current)
    return segments


def _resolve_voice_profile(voice_id: str | None):
    key = (voice_id or DEFAULT_VOICE).strip() or DEFAULT_VOICE
    return VOICE_PROFILES.get(key, VOICE_PROFILES[DEFAULT_VOICE])


def _load_reference_profile(voice_id: str | None):
    profile = _resolve_voice_profile(voice_id)
    ref_audio = Path(profile["ref_audio"])
    ref_text_path = Path(profile["ref_text"])
    if not ref_audio.exists():
        raise HTTPException(status_code=500, detail=f"missing reference audio: {ref_audio}")
    if not ref_text_path.exists():
        raise HTTPException(status_code=500, detail=f"missing reference text: {ref_text_path}")
    ref_text = ref_text_path.read_text(encoding="utf-8").strip()
    if not ref_text:
        raise HTTPException(status_code=500, detail=f"empty reference text: {ref_text_path}")
    return preprocess_ref_audio_text(str(ref_audio), ref_text)


@lru_cache(maxsize=1)
def load_runtime():
    model_cfg_path = str(files("f5_tts").joinpath(f"configs/{MODEL_NAME}.yaml"))
    model_cfg = OmegaConf.load(model_cfg_path)
    model_cls = get_class(f"f5_tts.model.{model_cfg.model.backbone}")
    model_arc = model_cfg.model.arch

    ckpt_file = str(cached_path(f"hf://{MODEL_REPO}/{MODEL_CKPT}", cache_dir=str(MODELS_DIR)))
    vocab_file = str(cached_path(f"hf://{MODEL_REPO}/{MODEL_VOCAB}", cache_dir=str(MODELS_DIR)))

    vocoder = load_vocoder(vocoder_name=VOCODER_NAME, is_local=False, local_path="", device=DEVICE)
    ema_model = load_model(
        model_cls,
        model_arc,
        ckpt_file,
        mel_spec_type=VOCODER_NAME,
        vocab_file=vocab_file,
        device=DEVICE,
    )
    return {"ema_model": ema_model, "vocoder": vocoder}


def _infer_audio(runtime, ref_audio, ref_text, text: str):
    audio_segment, sample_rate, _ = infer_process(
        ref_audio,
        ref_text,
        text,
        runtime["ema_model"],
        runtime["vocoder"],
        mel_spec_type=VOCODER_NAME,
        target_rms=0.1,
        cross_fade_duration=0.15,
        nfe_step=16,
        cfg_strength=2.0,
        sway_sampling_coef=-1.0,
        speed=1.0,
        fix_duration=None,
        device=DEVICE,
    )
    return audio_segment, sample_rate


@app.get("/health")
def health():
    return {
        "status": "ok",
        "provider": "f5-tts-german",
        "model_repo": MODEL_REPO,
        "model_name": MODEL_NAME,
        "default_voice": DEFAULT_VOICE,
        "device": DEVICE,
    }


@app.post("/v1/audio/speech")
def synthesize(request: SpeechRequest):
    text = _normalize_text(request.input)
    if not text:
        raise HTTPException(status_code=400, detail="input is required")

    try:
        runtime = load_runtime()
        ref_audio, ref_text = _load_reference_profile(request.voice)
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"load f5 runtime: {exc}") from exc

    segments = _segment_text(text)
    if not segments:
        raise HTTPException(status_code=400, detail="input is required")

    try:
        with INFERENCE_LOCK:
            rendered_segments = []
            sample_rate = None
            for segment in segments:
                segment_audio, segment_rate = _infer_audio(runtime, ref_audio, ref_text, segment)
                if sample_rate is None:
                    sample_rate = segment_rate
                rendered_segments.append(segment_audio)
            if not rendered_segments or sample_rate is None:
                raise RuntimeError("no audio segments rendered")
            audio_segment = np.concatenate(rendered_segments)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"f5 inference failed: {exc}") from exc

    output = io.BytesIO()
    sf.write(output, audio_segment, sample_rate, format="WAV")
    return Response(content=output.getvalue(), media_type="audio/wav")
