import os
import subprocess
import tempfile
from pathlib import Path
from typing import Any

import nemo.collections.asr as nemo_asr
import numpy as np
import soundfile as sf
from fastapi import FastAPI, File, HTTPException, UploadFile


app = FastAPI(title="AI Game Master Parakeet STT")

MODEL_NAME = os.getenv("STT_MODEL", "nvidia/parakeet-tdt-0.6b-v3")
model = None


def get_model():
    global model
    if model is None:
        model = nemo_asr.models.ASRModel.from_pretrained(model_name=MODEL_NAME)
    return model


def prepare_audio_for_transcription(source: Path) -> Path:
    prepared = source.with_name(f"{source.stem}_mono.wav")
    converted = source.with_name(f"{source.stem}_converted.wav")
    try:
        subprocess.run(
            [
                "ffmpeg",
                "-y",
                "-i",
                str(source),
                "-ac",
                "1",
                "-ar",
                "16000",
                str(converted),
            ],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        return converted
    except Exception:
        pass
    try:
        audio, sample_rate = sf.read(str(source), always_2d=False)
    except Exception:
        return source

    if isinstance(audio, np.ndarray) and audio.ndim == 2:
        audio = audio.mean(axis=1)
        sf.write(str(prepared), audio, sample_rate)
        return prepared
    return source


def extract_transcript_text(payload: Any) -> str:
    texts: list[str] = []

    def visit(value: Any) -> None:
        if value is None:
            return
        if isinstance(value, str):
            cleaned = value.strip()
            if cleaned and not cleaned.startswith("Hypothesis("):
                texts.append(cleaned)
            return
        if isinstance(value, (list, tuple)):
            for entry in value:
                visit(entry)
            return
        text_attr = getattr(value, "text", None)
        if isinstance(text_attr, str):
            cleaned = text_attr.strip()
            if cleaned:
                texts.append(cleaned)
            return

    visit(payload)
    merged = " ".join(texts)
    return " ".join(merged.split()).strip()


@app.get("/health")
def health():
    return {"status": "ok", "provider": "parakeet", "model": MODEL_NAME}


@app.post("/v1/audio/transcriptions")
async def transcribe(file: UploadFile = File(...)):
    suffix = Path(file.filename or "audio.wav").suffix or ".wav"
    with tempfile.TemporaryDirectory(prefix="parakeet-") as tmpdir:
        target = Path(tmpdir) / f"input{suffix}"
        target.write_bytes(await file.read())
        prepared_target = prepare_audio_for_transcription(target)
        try:
            loaded_model = get_model()
            result = loaded_model.transcribe([str(prepared_target)])
        except Exception as exc:
            raise HTTPException(status_code=500, detail=f"transcription failed: {exc}") from exc
    if not result:
        raise HTTPException(status_code=500, detail="empty transcription result")
    text = extract_transcript_text(result)
    if not text:
        raise HTTPException(status_code=422, detail="no speech detected")
    return {"text": text, "model": MODEL_NAME}
