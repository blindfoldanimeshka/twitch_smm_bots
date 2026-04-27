from fastapi import FastAPI, UploadFile, File, Form, HTTPException
from faster_whisper import WhisperModel
import os
import tempfile
from typing import Optional
import threading
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Local STT API", version="1.0.0")

MODEL_NAME = os.getenv("STT_MODEL_NAME", "small")
DEVICE = os.getenv("STT_DEVICE", "cpu")
COMPUTE_TYPE = os.getenv("STT_COMPUTE_TYPE", "int8")

model = None
model_lock = threading.Lock()


def get_model() -> WhisperModel:
    global model
    if model is not None:
        return model
    with model_lock:
        if model is None:
            logger.info(f"Loading Whisper model: {MODEL_NAME} on {DEVICE} with {COMPUTE_TYPE}")
            model = WhisperModel(MODEL_NAME, device=DEVICE, compute_type=COMPUTE_TYPE)
            logger.info("Whisper model loaded successfully")
    return model


@app.get("/health")
def health():
    return {
        "status": "ok",
        "model": MODEL_NAME,
        "device": DEVICE,
        "model_loaded": model is not None,
    }


@app.post("/v1/audio/transcriptions")
async def transcriptions(
    file: UploadFile = File(...),
    model_name: str = Form(..., alias="model"),
    language: Optional[str] = Form(default=None),
):
    logger.info(f"Received transcription request: file={file.filename}, language={language}")
    _ = model_name  # compatibility field for OpenAI-style API
    if not file.filename:
        raise HTTPException(status_code=400, detail="file is required")

    suffix = ".wav"
    if "." in file.filename:
        suffix = "." + file.filename.rsplit(".", 1)[-1]

    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
        content = await file.read()
        if not content:
            raise HTTPException(status_code=400, detail="empty file")
        tmp.write(content)
        tmp_path = tmp.name

    logger.info(f"Audio file saved to {tmp_path}, size: {len(content)} bytes")

    try:
        logger.info("Getting Whisper model...")
        stt_model = get_model()
        logger.info("Starting transcription...")
        segments, _info = stt_model.transcribe(
            tmp_path,
            language=language,
            vad_filter=True,
            beam_size=1,
        )
        text = " ".join(seg.text.strip() for seg in segments if seg.text.strip()).strip()
        logger.info(f"Transcription complete: {len(text)} chars")
        return {"text": text}
    except Exception as e:
        logger.error(f"Transcription error: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))
    finally:
        try:
            os.remove(tmp_path)
        except OSError:
            pass
