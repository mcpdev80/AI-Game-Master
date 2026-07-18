from __future__ import annotations

import base64
import io
import os
import re
import time
from pathlib import Path
from typing import Any

import cv2
import numpy as np
import pytesseract
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from PIL import Image


app = FastAPI(title="dungeon-master-vision")
DEBUG_SNAPSHOTS_DIR = Path(os.getenv("DEBUG_SNAPSHOTS_DIR", "/tmp/dm-vision-debug"))


class DetectDiceRequest(BaseModel):
    image_data_url: str
    language: str | None = "de"


class DiceResult(BaseModel):
    type: str
    value: int


class DiceBox(BaseModel):
    x: int
    y: int
    w: int
    h: int


class DetectDiceResponse(BaseModel):
    dice: list[DiceResult]
    dice_count: int
    boxes: list[DiceBox]
    confidence: float
    notes: str
    raw_model: str = "d6-opencv-ocr-mvp"


DATA_URL_RE = re.compile(r"^data:image/(?P<fmt>[a-zA-Z0-9.+-]+);base64,(?P<data>.+)$")


def decode_data_url(image_data_url: str) -> tuple[Image.Image, np.ndarray]:
    match = DATA_URL_RE.match(image_data_url)
    if not match:
        raise HTTPException(status_code=400, detail="image_data_url must be a base64 data URL")

    try:
        payload = base64.b64decode(match.group("data"))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"invalid base64 image payload: {exc}") from exc

    try:
        image = Image.open(io.BytesIO(payload)).convert("RGB")
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"invalid image payload: {exc}") from exc

    rgb = np.array(image)
    bgr = cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR)
    return image, bgr


def save_debug_snapshot(image: Image.Image) -> None:
    try:
        DEBUG_SNAPSHOTS_DIR.mkdir(parents=True, exist_ok=True)
        filename = DEBUG_SNAPSHOTS_DIR / f"dice-{int(time.time() * 1000)}.jpg"
        image.save(filename, format="JPEG", quality=92)
    except Exception:
        pass


def order_results(results: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return sorted(results, key=lambda item: (item["center"][1], item["center"][0]))


def box_iou(a: tuple[int, int, int, int], b: tuple[int, int, int, int]) -> float:
    ax0, ay0, ax1, ay1 = a
    bx0, by0, bx1, by1 = b
    inter_x0 = max(ax0, bx0)
    inter_y0 = max(ay0, by0)
    inter_x1 = min(ax1, bx1)
    inter_y1 = min(ay1, by1)
    inter_w = max(0, inter_x1 - inter_x0)
    inter_h = max(0, inter_y1 - inter_y0)
    inter_area = inter_w * inter_h
    if inter_area <= 0:
        return 0.0
    area_a = max(1, (ax1 - ax0) * (ay1 - ay0))
    area_b = max(1, (bx1 - bx0) * (by1 - by0))
    return inter_area / float(area_a + area_b - inter_area)


def center_distance(a: tuple[int, int, int, int], b: tuple[int, int, int, int]) -> float:
    acx = (a[0] + a[2]) / 2.0
    acy = (a[1] + a[3]) / 2.0
    bcx = (b[0] + b[2]) / 2.0
    bcy = (b[1] + b[3]) / 2.0
    return float(((acx - bcx) ** 2 + (acy - bcy) ** 2) ** 0.5)


def dedupe_by_position(results: list[dict[str, Any]]) -> list[dict[str, Any]]:
    kept: list[dict[str, Any]] = []
    for item in sorted(results, key=lambda result: result["confidence"], reverse=True):
        item_box = item["box"]
        item_size = max(item_box[2] - item_box[0], item_box[3] - item_box[1])
        if any(
            box_iou(item_box, existing["box"]) >= 0.28
            or center_distance(item_box, existing["box"]) < max(18.0, item_size * 0.33)
            for existing in kept
        ):
            continue
        kept.append(item)
    return kept


def warp_from_rect(image: np.ndarray, rect: tuple[tuple[float, float], tuple[float, float], float]) -> tuple[np.ndarray | None, tuple[int, int, int, int]]:
    (cx, cy), (width, height), _ = rect
    if width < 20 or height < 20:
        return None, (0, 0, 0, 0)

    box = cv2.boxPoints(rect).astype("float32")
    side = int(max(width, height))
    if side < 24:
        return None, (0, 0, 0, 0)

    dst = np.array([[0, 0], [side - 1, 0], [side - 1, side - 1], [0, side - 1]], dtype="float32")
    matrix = cv2.getPerspectiveTransform(order_box_points(box), dst)
    warped = cv2.warpPerspective(image, matrix, (side, side))

    x0 = max(0, int(cx - width / 2) - 8)
    y0 = max(0, int(cy - height / 2) - 8)
    x1 = min(image.shape[1], int(cx + width / 2) + 8)
    y1 = min(image.shape[0], int(cy + height / 2) + 8)
    return warped, (x0, y0, x1, y1)


def square_crop(image: np.ndarray, center: tuple[int, int], side: int) -> tuple[np.ndarray | None, tuple[int, int, int, int]]:
    half = side // 2
    cx, cy = center
    x0 = max(0, cx - half)
    y0 = max(0, cy - half)
    x1 = min(image.shape[1], cx + half)
    y1 = min(image.shape[0], cy + half)
    if x1 - x0 < 24 or y1 - y0 < 24:
        return None, (0, 0, 0, 0)
    return image[y0:y1, x0:x1].copy(), (x0, y0, x1, y1)


def order_box_points(pts: np.ndarray) -> np.ndarray:
    rect = np.zeros((4, 2), dtype="float32")
    s = pts.sum(axis=1)
    rect[0] = pts[np.argmin(s)]
    rect[2] = pts[np.argmax(s)]
    diff = np.diff(pts, axis=1)
    rect[1] = pts[np.argmin(diff)]
    rect[3] = pts[np.argmax(diff)]
    return rect


def find_die_candidates(image: np.ndarray) -> list[tuple[np.ndarray, tuple[int, int, int, int], tuple[int, int]]]:
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    blur = cv2.GaussianBlur(gray, (5, 5), 0)
    image_area = image.shape[0] * image.shape[1]
    kernel = np.ones((5, 5), np.uint8)
    _, otsu_bright = cv2.threshold(blur, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)
    _, otsu_dark = cv2.threshold(blur, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
    variants = [
        cv2.threshold(blur, 118, 255, cv2.THRESH_BINARY)[1],
        cv2.threshold(blur, 92, 255, cv2.THRESH_BINARY_INV)[1],
        otsu_bright,
        otsu_dark,
        cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 31, 5),
        cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY_INV, 31, 7),
        cv2.Canny(blur, 40, 140),
    ]

    raw_candidates: list[tuple[np.ndarray, tuple[int, int, int, int], tuple[int, int], float]] = []
    for variant in variants:
        merged = cv2.morphologyEx(variant, cv2.MORPH_CLOSE, kernel, iterations=2)
        merged = cv2.dilate(merged, kernel, iterations=1)
        contours, _ = cv2.findContours(merged, cv2.RETR_LIST, cv2.CHAIN_APPROX_SIMPLE)

        for contour in contours:
            area = cv2.contourArea(contour)
            if area < image_area * 0.0015 or area > image_area * 0.16:
                continue

            perimeter = cv2.arcLength(contour, True)
            if perimeter <= 0:
                continue
            approx = cv2.approxPolyDP(contour, 0.04 * perimeter, True)
            if len(approx) < 4 or len(approx) > 10:
                continue

            rect = cv2.minAreaRect(contour)
            (_, _), (w, h), _ = rect
            if w <= 0 or h <= 0:
                continue
            ratio = max(w, h) / max(1.0, min(w, h))
            if ratio > 1.8:
                continue

            solidity = area / max(1.0, cv2.contourArea(cv2.convexHull(contour)))
            if solidity < 0.62:
                continue

            warped, box = warp_from_rect(image, rect)
            if warped is None:
                continue
            box_w = box[2] - box[0]
            box_h = box[3] - box[1]
            if box_w < 34 or box_h < 34:
                continue
            margin_x = int(image.shape[1] * 0.045)
            margin_y = int(image.shape[0] * 0.045)
            if (
                box[0] < margin_x
                or box[1] < margin_y
                or box[2] > image.shape[1] - margin_x
                or box[3] > image.shape[0] - margin_y
            ):
                continue
            center = (int((box[0] + box[2]) / 2), int((box[1] + box[3]) / 2))
            score = float(area * min(solidity, 1.0))
            raw_candidates.append((warped, box, center, score))

            box_w = box[2] - box[0]
            box_h = box[3] - box[1]
            if 60 <= min(box_w, box_h) <= 140:
                expanded_side = int(max(box_w, box_h) * 1.9)
                expanded_face, expanded_box = square_crop(image, center, expanded_side)
                if expanded_face is not None:
                    expanded_score = score * 0.92
                    raw_candidates.append((expanded_face, expanded_box, center, expanded_score))

    deduped: list[tuple[np.ndarray, tuple[int, int, int, int], tuple[int, int], float]] = []
    for candidate in sorted(raw_candidates, key=lambda item: item[3], reverse=True):
        _, box, _, _ = candidate
        size = max(box[2] - box[0], box[3] - box[1])
        if any(
            box_iou(box, existing[1]) >= 0.2
            or center_distance(box, existing[1]) < max(16.0, size * 0.28)
            for existing in deduped
        ):
            continue
        deduped.append(candidate)

    if not deduped:
        return []

    sizes = sorted(max(box[2] - box[0], box[3] - box[1]) for _, box, _, _ in deduped)
    median_size = sizes[len(sizes) // 2]
    filtered = [
        candidate
        for candidate in deduped
        if max(candidate[1][2] - candidate[1][0], candidate[1][3] - candidate[1][1]) >= max(70, int(median_size * 0.62))
        and max(candidate[1][2] - candidate[1][0], candidate[1][3] - candidate[1][1]) <= int(median_size * 1.75)
    ]

    strongest = sorted(filtered or deduped, key=lambda item: item[3], reverse=True)[:6]
    return [(warped, box, center) for warped, box, center, _ in strongest]


def detect_pips(face: np.ndarray) -> tuple[int | None, float]:
    gray = cv2.cvtColor(face, cv2.COLOR_BGR2GRAY)
    side = min(gray.shape[0], gray.shape[1])
    inset = max(4, int(side * 0.08))
    roi = gray[inset: gray.shape[0] - inset, inset: gray.shape[1] - inset]
    blur = cv2.GaussianBlur(roi, (5, 5), 0)
    face_area = face.shape[0] * face.shape[1]
    pip_counts: list[int] = []

    thresholds = [
        cv2.threshold(blur, 145, 255, cv2.THRESH_BINARY_INV)[1],
        cv2.threshold(blur, 130, 255, cv2.THRESH_BINARY_INV)[1],
        cv2.adaptiveThreshold(blur, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY_INV, 21, 5),
    ]

    for thresh in thresholds:
        opened = cv2.morphologyEx(thresh, cv2.MORPH_OPEN, np.ones((3, 3), np.uint8), iterations=1)
        contours, _ = cv2.findContours(opened, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        pips = 0

        for contour in contours:
            area = cv2.contourArea(contour)
            if area < face_area * 0.002 or area > face_area * 0.08:
                continue
            perimeter = cv2.arcLength(contour, True)
            if perimeter <= 0:
                continue
            circularity = (4.0 * np.pi * area) / max(1.0, perimeter * perimeter)
            x, y, w, h = cv2.boundingRect(contour)
            if w < 4 or h < 4:
                continue
            ratio = w / float(h)
            fill = area / float(max(1, w * h))
            if 0.55 <= ratio <= 1.5 and 0.28 <= fill <= 0.92 and circularity >= 0.45:
                pips += 1

        if 1 <= pips <= 6:
            pip_counts.append(pips)

    if pip_counts:
        counts = {value: pip_counts.count(value) for value in set(pip_counts)}
        best_value = max(counts, key=lambda value: (counts[value], value))
        confidence = 0.74 + (0.06 if counts[best_value] > 1 else 0.0)
        return best_value, min(confidence, 0.88)
    return None, 0.0


def extract_digits(text: str) -> int | None:
    digits = re.sub(r"\D+", "", text)
    if not digits:
        return None
    try:
        return int(digits)
    except ValueError:
        return None


def ocr_d6_value(face: np.ndarray) -> tuple[int | None, float]:
    gray = cv2.cvtColor(face, cv2.COLOR_BGR2GRAY)
    side = min(gray.shape[0], gray.shape[1])
    inset = max(6, int(side * 0.16))
    gray = gray[inset: gray.shape[0] - inset, inset: gray.shape[1] - inset]
    best_value: int | None = None
    best_confidence = 0.0

    for rotation in (0, 90, 180, 270):
        rotated = cv2.rotate(gray, {
            0: cv2.ROTATE_90_CLOCKWISE,  # placeholder, overwritten below
        }.get(rotation, cv2.ROTATE_90_CLOCKWISE)) if rotation else gray
        if rotation == 180:
            rotated = cv2.rotate(gray, cv2.ROTATE_180)
        elif rotation == 270:
            rotated = cv2.rotate(gray, cv2.ROTATE_90_COUNTERCLOCKWISE)
        elif rotation == 90:
            rotated = cv2.rotate(gray, cv2.ROTATE_90_CLOCKWISE)

        enlarged = cv2.resize(rotated, None, fx=4.0, fy=4.0, interpolation=cv2.INTER_CUBIC)
        for variant in (
            cv2.threshold(enlarged, 160, 255, cv2.THRESH_BINARY)[1],
            cv2.threshold(enlarged, 160, 255, cv2.THRESH_BINARY_INV)[1],
            cv2.adaptiveThreshold(enlarged, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 31, 6),
            cv2.adaptiveThreshold(enlarged, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY_INV, 31, 6),
        ):
            data = pytesseract.image_to_data(
                variant,
                config="--psm 10 --oem 3 -c tessedit_char_whitelist=123456",
                output_type=pytesseract.Output.DICT,
            )
            for raw_text, raw_confidence in zip(data.get("text", []), data.get("conf", [])):
                value = extract_digits(raw_text)
                if value is None or value < 1 or value > 6:
                    continue
                try:
                    confidence = max(0.0, min(99.0, float(raw_confidence))) / 100.0
                except (TypeError, ValueError):
                    confidence = 0.45
                if confidence >= best_confidence:
                    best_value = value
                    best_confidence = confidence

    return best_value, best_confidence


def classify_d6(face: np.ndarray) -> tuple[int | None, float]:
    pip_value, pip_confidence = detect_pips(face)
    if pip_value is not None:
        return pip_value, pip_confidence

    ocr_value, ocr_confidence = ocr_d6_value(face)
    if ocr_value is not None:
        return ocr_value, max(0.45, ocr_confidence)

    return None, 0.0


def detect_dice(image: np.ndarray) -> tuple[list[dict[str, Any]], list[dict[str, int]], str, float]:
    candidates = find_die_candidates(image)
    boxes = [
        {"x": box[0], "y": box[1], "w": box[2] - box[0], "h": box[3] - box[1]}
        for _, box, _ in candidates
    ]
    results: list[dict[str, Any]] = []

    for face, box, center in candidates:
        value, confidence = classify_d6(face)
        if value is None:
            continue
        results.append({
            "type": "d6",
            "value": value,
            "confidence": confidence,
            "box": box,
            "center": center,
        })

    if not results:
        return [], boxes, "No clear d6 detected. Use a top-down view, keep the dice inside the center area, reduce glare, and keep them still for a moment.", 0.0

    positioned = dedupe_by_position(results)
    ordered = order_results(positioned)
    mean_confidence = float(sum(item["confidence"] for item in ordered) / len(ordered))
    compacted = [{"type": item["type"], "value": item["value"]} for item in ordered]
    return compacted, boxes, f"{len(boxes)} d6 candidates found, {len(compacted)} values read by the OpenCV/OCR MVP.", mean_confidence


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "service": "dungeon-master-vision"}


@app.post("/detect/dice", response_model=DetectDiceResponse)
def detect_dice_route(payload: DetectDiceRequest) -> DetectDiceResponse:
    image, bgr = decode_data_url(payload.image_data_url)
    save_debug_snapshot(image)
    dice, boxes, notes, confidence = detect_dice(bgr)
    return DetectDiceResponse(
        dice=[DiceResult(**item) for item in dice],
        dice_count=len(boxes),
        boxes=[DiceBox(**item) for item in boxes],
        confidence=confidence,
        notes=notes,
    )
