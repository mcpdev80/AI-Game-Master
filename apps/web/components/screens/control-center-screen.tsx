"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { Brain, Camera, Database, Dices, Mic, Monitor, Network, PlayCircle, Radio, RefreshCw, Square, Volume2, Wifi, X } from "lucide-react";
import { PageIntro, Panel, StatCard, StatusPill } from "../studio-primitives";
import { createFungalCavernsDemo, detectDiceFromImage, fetchLLMModels, stabilizeDiceFrames, testLLMConnection, updateSystemConfig, type DiceBox, type DiceDetection, type DiceDetectionFrame, type LLMGatewayStatus } from "../../lib/api";
import { createClientId } from "../../lib/client-id";
import type { PlayerLinkSlot, Session } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

type ControlCenterScreenProps = {
  services: { name: string; status: string }[];
  counts: Record<string, number>;
  llm: { base_url?: string; model?: string };
	tts: { provider?: string; model?: string };
	stt: { provider?: string; model?: string };
  llmGateway?: LLMGatewayStatus;
  sessions: Session[];
  playerLinks: PlayerLinkSlot[];
};

const cameraPreferenceStorageKey = "dm.camera.preferredDeviceId";
const microphonePreferenceStorageKey = "dm.audio.preferredInputId";
const speakerPreferenceStorageKey = "dm.audio.preferredOutputId";
type DiceType = "d4" | "d6" | "d8" | "d10" | "d12" | "d20" | "d100";
const defaultDiceFrameDraft: DiceDetection[] = [];
type GuidedRollStep = {
  id: string;
  type: DiceType;
  count: number;
  label: string;
  status: "pending" | "active" | "confirmed";
  confirmedValues: number[];
};

function diceSignature(dice: DiceDetection[]) {
  return [...dice]
    .sort((left, right) => {
      if (left.type === right.type) {
        return left.value - right.value;
      }
      return left.type.localeCompare(right.type);
    })
    .map((die) => `${die.type}:${die.value}`)
    .join("|");
}

function randomInt(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function diceTypeMaxValue(type: DiceType) {
  switch (type) {
    case "d4":
      return 4;
    case "d6":
      return 6;
    case "d8":
      return 8;
    case "d10":
      return 10;
    case "d12":
      return 12;
    case "d20":
      return 20;
    case "d100":
      return 100;
  }
}

function generateGuidedRollPlan(locale: "en" | "de"): GuidedRollStep[] {
  const labels = locale === "de"
    ? ["Angriffswurf", "Schadenswurf", "Rettungswurf", "Zaubereffekt", "Probe"]
    : ["attack roll", "damage roll", "saving throw", "spell effect", "check"];
  const types: DiceType[] = ["d4", "d6", "d8", "d10", "d12", "d20", "d100"];
  const stepCount = randomInt(1, 2);
  const steps: GuidedRollStep[] = [];

  for (let index = 0; index < stepCount; index += 1) {
    const type = types[randomInt(0, types.length - 1)];
    const count = type === "d20" ? randomInt(1, 2) : type === "d100" ? 1 : randomInt(1, 4);
    steps.push({
      id: createClientId("guided-roll"),
      type,
      count,
      label: labels[randomInt(0, labels.length - 1)],
      status: index === 0 ? "active" : "pending",
      confirmedValues: [],
    });
  }

  return steps;
}

export function ControlCenterScreen({ services, counts, llm, llmGateway, sessions, playerLinks, stt, tts }: ControlCenterScreenProps) {
  const { locale, tr } = useI18n();
  const checks = [
    { name: tr("Database", "Datenbank"), icon: Database, detail: tr("Postgres connected", "Postgres verbunden"), tone: "ready" as const },
    { name: tr("Player Screen", "Spieleransicht"), icon: Monitor, detail: tr("Second display route ready", "Zweiter Bildschirm bereit"), tone: "ready" as const },
    { name: tr("Player Portal", "Spielerportal"), icon: Wifi, detail: tr("LAN access enabled", "LAN-Zugriff aktiviert"), tone: "ready" as const },
    { name: tr("Network", "Netzwerk"), icon: Network, detail: tr("Local network reachable", "Lokales Netzwerk erreichbar"), tone: "ready" as const },
  ];
  const statusLabel = (status: string) => ({
    idle: tr("idle", "bereit"), ready: tr("ready", "bereit"), error: tr("error", "Fehler"), unsupported: tr("unsupported", "nicht unterstützt"),
    stabilizing: tr("stabilizing", "Stabilisierung"), stable: tr("stable", "stabil"), pending: tr("pending", "ausstehend"), active: tr("active", "aktiv"),
    confirmed: tr("confirmed", "bestätigt"), saved: tr("saved", "gespeichert"), missing: tr("missing", "fehlt"), success: tr("success", "erfolgreich"), running: tr("running", "läuft"),
    live: tr("live", "live"), paused: tr("paused", "pausiert"), ready_to_start: tr("ready to start", "startbereit"), ended: tr("ended", "beendet"),
  }[status] ?? status);
  const onlineServices = services.filter((service) => service.status === "online").length;
  const liveSession = sessions.find((session) => session.status === "live") ?? sessions[0] ?? null;
  const joinedPlayers = playerLinks.filter((slot) => slot.player_slot.status === "joined").length;
  const readyTone =
    llm.model && counts.documents > 0 && counts.campaigns > 0 && liveSession ? "ready" : "warning";
  const [cameraDevices, setCameraDevices] = useState<MediaDeviceInfo[]>([]);
  const [selectedCameraId, setSelectedCameraId] = useState("");
  const [savedCameraId, setSavedCameraId] = useState("");
  const [cameraStatus, setCameraStatus] = useState<"idle" | "ready" | "error" | "unsupported">("idle");
  const [cameraMessage, setCameraMessage] = useState(() => tr("No browser camera test has run yet.", "Noch kein Kameratest im Browser durchgeführt."));
  const [cameraSaveNotice, setCameraSaveNotice] = useState("");
  const [isTestingCamera, setIsTestingCamera] = useState(false);
  const [isCameraModalOpen, setIsCameraModalOpen] = useState(false);
  const [isDiceTestActive, setIsDiceTestActive] = useState(false);
  const [diceFrameDraft, setDiceFrameDraft] = useState<DiceDetection[]>(defaultDiceFrameDraft);
  const [showDiceDebugInput, setShowDiceDebugInput] = useState(false);
  const [diceFrameHistory, setDiceFrameHistory] = useState<DiceDetectionFrame[]>([]);
  const [detectedDice, setDetectedDice] = useState<DiceDetection[]>([]);
  const [detectedDiceCount, setDetectedDiceCount] = useState(0);
  const [detectedDiceBoxes, setDetectedDiceBoxes] = useState<DiceBox[]>([]);
  const [diceAnalysisImage, setDiceAnalysisImage] = useState<string | null>(null);
  const [diceAnalysisSize, setDiceAnalysisSize] = useState<{ width: number; height: number } | null>(null);
  const [diceStabilityStatus, setDiceStabilityStatus] = useState<"idle" | "stabilizing" | "stable" | "error">("idle");
  const [diceTestMessage, setDiceTestMessage] = useState(() => tr("No dice test has been run yet.", "Noch kein Würfeltest durchgeführt."));
  const [isCapturingDice, setIsCapturingDice] = useState(false);
  const [guidedRollPlan, setGuidedRollPlan] = useState<GuidedRollStep[]>([]);
  const [editableDetectedDice, setEditableDetectedDice] = useState<number[]>([]);
  const [microphoneDevices, setMicrophoneDevices] = useState<MediaDeviceInfo[]>([]);
  const [speakerDevices, setSpeakerDevices] = useState<MediaDeviceInfo[]>([]);
  const [selectedMicrophoneId, setSelectedMicrophoneId] = useState("");
  const [selectedSpeakerId, setSelectedSpeakerId] = useState("");
  const [savedMicrophoneId, setSavedMicrophoneId] = useState("");
  const [savedSpeakerId, setSavedSpeakerId] = useState("");
  const [audioStatus, setAudioStatus] = useState<"idle" | "ready" | "error" | "unsupported">("idle");
  const [audioMessage, setAudioMessage] = useState(() => tr("No microphone or speaker test has run yet.", "Noch kein Mikrofon- oder Lautsprechertest durchgeführt."));
  const [audioSaveNotice, setAudioSaveNotice] = useState("");
  const [isTestingMicrophone, setIsTestingMicrophone] = useState(false);
  const [isAudioModalOpen, setIsAudioModalOpen] = useState(false);
  const [micLevel, setMicLevel] = useState(0);
  const [llmTestStatus, setLlmTestStatus] = useState<"idle" | "running" | "success" | "error">("idle");
  const [llmTestMessage, setLlmTestMessage] = useState(() => tr("No model test has been run yet.", "Noch kein Modelltest durchgeführt."));
  const [llmBaseUrl, setLlmBaseUrl] = useState(llm.base_url ?? "");
  const [savedLlmBaseUrl, setSavedLlmBaseUrl] = useState(llm.base_url ?? "");
  const [llmModelInput, setLlmModelInput] = useState(llm.model ?? "");
  const [savedLlmModel, setSavedLlmModel] = useState(llm.model ?? "");
  const [availableModels, setAvailableModels] = useState<string[]>([]);
  const [llmSaveNotice, setLlmSaveNotice] = useState("");
  const [isLlmModalOpen, setIsLlmModalOpen] = useState(false);
  const [playerScreenTestStatus, setPlayerScreenTestStatus] = useState<"idle" | "success">("idle");
  const [playerScreenTestMessage, setPlayerScreenTestMessage] = useState(() => tr("No player screen test has been triggered yet.", "Noch kein Test der Spieleransicht ausgelöst."));
  const [demoStatus, setDemoStatus] = useState<"idle" | "creating" | "error">("idle");
  const [demoError, setDemoError] = useState("");
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const captureCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const audioStreamRef = useRef<MediaStream | null>(null);
  const audioContextRef = useRef<AudioContext | null>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const animationFrameRef = useRef<number | null>(null);
  const testAudioRef = useRef<HTMLAudioElement | null>(null);
  const cameraConfigured = savedCameraId.trim().length > 0;
  const audioConfigured = savedMicrophoneId.trim().length > 0 || savedSpeakerId.trim().length > 0;

  useEffect(() => {
    if (cameraStatus === "idle") setCameraMessage(tr("No browser camera test has run yet.", "Noch kein Kameratest im Browser durchgeführt."));
    if (diceStabilityStatus === "idle") setDiceTestMessage(tr("No dice test has been run yet.", "Noch kein Würfeltest durchgeführt."));
    if (audioStatus === "idle") setAudioMessage(tr("No microphone or speaker test has run yet.", "Noch kein Mikrofon- oder Lautsprechertest durchgeführt."));
    if (llmTestStatus === "idle") setLlmTestMessage(tr("No model test has been run yet.", "Noch kein Modelltest durchgeführt."));
    if (playerScreenTestStatus === "idle") setPlayerScreenTestMessage(tr("No player screen test has been triggered yet.", "Noch kein Test der Spieleransicht ausgelöst."));
  }, [audioStatus, cameraStatus, diceStabilityStatus, llmTestStatus, locale, playerScreenTestStatus, tr]);

  const selectedCameraLabel = useMemo(
    () => cameraDevices.find((device) => device.deviceId === selectedCameraId)?.label || tr("Default browser camera", "Standard-Browserkamera"),
    [cameraDevices, selectedCameraId, tr]
  );
  const savedCameraLabel = useMemo(
    () => cameraDevices.find((device) => device.deviceId === savedCameraId)?.label || (cameraConfigured ? tr("Saved camera", "Gespeicherte Kamera") : tr("Not configured", "Nicht konfiguriert")),
    [cameraConfigured, cameraDevices, savedCameraId, tr]
  );
  const selectedMicrophoneLabel = useMemo(
    () => microphoneDevices.find((device) => device.deviceId === selectedMicrophoneId)?.label || tr("Default microphone", "Standardmikrofon"),
    [microphoneDevices, selectedMicrophoneId, tr]
  );
  const selectedSpeakerLabel = useMemo(
    () => speakerDevices.find((device) => device.deviceId === selectedSpeakerId)?.label || tr("Default browser output", "Standard-Browserausgabe"),
    [speakerDevices, selectedSpeakerId, tr]
  );
  const savedMicrophoneLabel = useMemo(
    () =>
      microphoneDevices.find((device) => device.deviceId === savedMicrophoneId)?.label ||
      (savedMicrophoneId ? tr("Saved microphone", "Gespeichertes Mikrofon") : tr("No microphone saved", "Kein Mikrofon gespeichert")),
    [microphoneDevices, savedMicrophoneId, tr]
  );
  const savedSpeakerLabel = useMemo(
    () =>
      speakerDevices.find((device) => device.deviceId === savedSpeakerId)?.label ||
      (savedSpeakerId ? tr("Saved speaker", "Gespeicherter Lautsprecher") : tr("No speaker saved", "Kein Lautsprecher gespeichert")),
    [savedSpeakerId, speakerDevices, tr]
  );

  function stopCameraStream() {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
    if (videoRef.current) {
      videoRef.current.srcObject = null;
    }
  }

  function resetDiceTest() {
    setIsDiceTestActive(false);
    setGuidedRollPlan([]);
    setDiceFrameHistory([]);
    setDetectedDice([]);
    setDetectedDiceCount(0);
    setEditableDetectedDice([]);
    setDetectedDiceBoxes([]);
    setDiceAnalysisImage(null);
    setDiceAnalysisSize(null);
    setDiceStabilityStatus("idle");
    setDiceTestMessage(tr("No dice test has been run yet.", "Noch kein Würfeltest durchgeführt."));
    setDiceFrameDraft(defaultDiceFrameDraft);
    setShowDiceDebugInput(false);
  }

  const activeGuidedRollStep = guidedRollPlan.find((step) => step.status === "active") ?? null;

  function captureCurrentFrame(): string | null {
    const video = videoRef.current;
    const canvas = captureCanvasRef.current;
    if (!video || !canvas || video.videoWidth === 0 || video.videoHeight === 0) {
      return null;
    }

    const sourceWidth = video.videoWidth;
    const sourceHeight = video.videoHeight;
    const cropWidth = Math.round(sourceWidth * 0.9);
    const cropHeight = Math.round(sourceHeight * 0.9);
    const cropX = Math.max(0, Math.round((sourceWidth - cropWidth) / 2));
    const cropY = Math.max(0, Math.round((sourceHeight - cropHeight) / 2));
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

  function sleep(ms: number) {
    return new Promise((resolve) => window.setTimeout(resolve, ms));
  }

  function stopMicrophoneTest() {
    if (animationFrameRef.current) {
      window.cancelAnimationFrame(animationFrameRef.current);
      animationFrameRef.current = null;
    }
    if (audioStreamRef.current) {
      audioStreamRef.current.getTracks().forEach((track) => track.stop());
      audioStreamRef.current = null;
    }
    if (audioContextRef.current) {
      void audioContextRef.current.close();
      audioContextRef.current = null;
    }
    analyserRef.current = null;
    setMicLevel(0);
    setAudioStatus("idle");
    setAudioMessage(tr("Microphone test stopped.", "Mikrofontest beendet."));
  }

  useEffect(() => {
    if (typeof window !== "undefined") {
      const storedDeviceId = window.localStorage.getItem(cameraPreferenceStorageKey) ?? "";
      setSavedCameraId(storedDeviceId);
      setSelectedCameraId(storedDeviceId);
      const storedMicrophoneId = window.localStorage.getItem(microphonePreferenceStorageKey) ?? "";
      const storedSpeakerId = window.localStorage.getItem(speakerPreferenceStorageKey) ?? "";
      setSavedMicrophoneId(storedMicrophoneId);
      setSelectedMicrophoneId(storedMicrophoneId);
      setSavedSpeakerId(storedSpeakerId);
      setSelectedSpeakerId(storedSpeakerId);
    }
    return () => {
      stopCameraStream();
      stopMicrophoneTest();
    };
  }, []);

  useEffect(() => {
    setSavedLlmBaseUrl(llm.base_url ?? "");
    setLlmBaseUrl(llm.base_url ?? "");
    setSavedLlmModel(llm.model ?? "");
    setLlmModelInput(llm.model ?? "");
  }, [llm.base_url, llm.model]);

  async function refreshCameraDevices() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.enumerateDevices) {
      setCameraStatus("unsupported");
      setCameraMessage(tr("This browser does not expose camera device enumeration.", "Dieser Browser stellt keine Kamerageräteliste bereit."));
      return;
    }
    const devices = await navigator.mediaDevices.enumerateDevices();
    const cameras = devices.filter((device) => device.kind === "videoinput");
    setCameraDevices(cameras);
    if (!selectedCameraId && cameras[0]?.deviceId) {
      setSelectedCameraId(savedCameraId || cameras[0].deviceId);
    }
    if (cameras.length === 0) {
      setCameraStatus("error");
      setCameraMessage(tr("No camera devices were found in this browser.", "In diesem Browser wurden keine Kameras gefunden."));
    }
  }

  async function refreshAudioDevices() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.enumerateDevices) {
      setAudioStatus("unsupported");
      setAudioMessage(tr("This browser does not expose audio device enumeration.", "Dieser Browser stellt keine Audiogeräteliste bereit."));
      return;
    }
    const devices = await navigator.mediaDevices.enumerateDevices();
    const microphones = devices.filter((device) => device.kind === "audioinput");
    const speakers = devices.filter((device) => device.kind === "audiooutput");
    setMicrophoneDevices(microphones);
    setSpeakerDevices(speakers);
    if (!selectedMicrophoneId && microphones[0]?.deviceId) {
      setSelectedMicrophoneId(savedMicrophoneId || microphones[0].deviceId);
    }
    if (!selectedSpeakerId && speakers[0]?.deviceId) {
      setSelectedSpeakerId(savedSpeakerId || speakers[0].deviceId);
    }
    if (microphones.length === 0 && speakers.length === 0) {
      setAudioStatus("error");
      setAudioMessage(tr("No audio devices were found in this browser.", "In diesem Browser wurden keine Audiogeräte gefunden."));
    }
  }

  async function handleCameraTest() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setCameraStatus("unsupported");
      setCameraMessage(tr("This browser does not support camera access.", "Dieser Browser unterstützt keinen Kamerazugriff."));
      return;
    }

    setIsTestingCamera(true);
    setCameraMessage(tr("Requesting camera access...", "Kamerazugriff wird angefordert …"));
    try {
      stopCameraStream();
      const stream = await navigator.mediaDevices.getUserMedia({
        video: selectedCameraId ? { deviceId: { exact: selectedCameraId } } : true,
        audio: false,
      });
      streamRef.current = stream;
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
      }
      await refreshCameraDevices();
      setCameraStatus("ready");
      setCameraMessage(tr(`Live preview active from ${selectedCameraLabel}.`, `Live-Vorschau von ${selectedCameraLabel} aktiv.`));
    } catch (error) {
      setCameraStatus("error");
      setCameraMessage(error instanceof Error ? error.message : tr("Camera access failed.", "Kamerazugriff fehlgeschlagen."));
    } finally {
      setIsTestingCamera(false);
    }
  }

  function handleStopCameraTest() {
    stopCameraStream();
    setCameraStatus("idle");
    setCameraMessage(tr("Camera test stopped. The browser stream was released.", "Kameratest beendet. Der Browserstream wurde freigegeben."));
  }

  async function handleMicrophoneTest() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setAudioStatus("unsupported");
      setAudioMessage(tr("This browser does not support microphone access.", "Dieser Browser unterstützt keinen Mikrofonzugriff."));
      return;
    }

    setIsTestingMicrophone(true);
    setAudioMessage(tr("Requesting microphone access...", "Mikrofonzugriff wird angefordert …"));
    try {
      stopMicrophoneTest();
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: selectedMicrophoneId ? { deviceId: { exact: selectedMicrophoneId } } : true,
        video: false,
      });
      audioStreamRef.current = stream;
      const audioContext = new window.AudioContext();
      const analyser = audioContext.createAnalyser();
      analyser.fftSize = 256;
      const source = audioContext.createMediaStreamSource(stream);
      source.connect(analyser);
      audioContextRef.current = audioContext;
      analyserRef.current = analyser;
      const data = new Uint8Array(analyser.frequencyBinCount);

      const tick = () => {
        if (!analyserRef.current) return;
        analyserRef.current.getByteTimeDomainData(data);
        let peak = 0;
        for (const value of data) {
          peak = Math.max(peak, Math.abs(value - 128));
        }
        setMicLevel(Math.min(100, Math.round((peak / 128) * 160)));
        animationFrameRef.current = window.requestAnimationFrame(tick);
      };

      animationFrameRef.current = window.requestAnimationFrame(tick);
      await refreshAudioDevices();
      setAudioStatus("ready");
      setAudioMessage(tr(`Microphone live from ${selectedMicrophoneLabel}.`, `Mikrofon ${selectedMicrophoneLabel} ist aktiv.`));
    } catch (error) {
      setAudioStatus("error");
      setAudioMessage(error instanceof Error ? error.message : tr("Microphone access failed.", "Mikrofonzugriff fehlgeschlagen."));
    } finally {
      setIsTestingMicrophone(false);
    }
  }

  async function handleSpeakerTest() {
    try {
      if (!testAudioRef.current) {
        testAudioRef.current = new Audio(
          "data:audio/wav;base64,UklGRlQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YTAAAAAAAP//AAD//wAA//8AAP//AAD//wAA"
        );
      }
      const audio = testAudioRef.current;
      const maybeSink = audio as HTMLAudioElement & { setSinkId?: (deviceId: string) => Promise<void> };
      if (selectedSpeakerId && typeof maybeSink.setSinkId === "function") {
        await maybeSink.setSinkId(selectedSpeakerId);
      }
      audio.currentTime = 0;
      await audio.play();
      setAudioStatus("ready");
      setAudioMessage(tr(`Speaker test played on ${selectedSpeakerLabel}.`, `Lautsprechertest über ${selectedSpeakerLabel} abgespielt.`));
    } catch (error) {
      setAudioStatus("error");
      setAudioMessage(error instanceof Error ? error.message : tr("Speaker test failed.", "Lautsprechertest fehlgeschlagen."));
    }
  }

  function handleOpenCameraModal() {
    setIsCameraModalOpen(true);
    void refreshCameraDevices();
  }

  function handleOpenAudioModal() {
    setIsAudioModalOpen(true);
    void refreshAudioDevices();
  }

  function handleOpenLlmModal() {
    setIsLlmModalOpen(true);
  }

  function handleCloseCameraModal() {
    handleStopCameraTest();
    resetDiceTest();
    setSelectedCameraId(savedCameraId);
    setCameraSaveNotice("");
    setIsCameraModalOpen(false);
  }

  function handleCloseAudioModal() {
    stopMicrophoneTest();
    setSelectedMicrophoneId(savedMicrophoneId);
    setSelectedSpeakerId(savedSpeakerId);
    setAudioSaveNotice("");
    setIsAudioModalOpen(false);
  }

  function handleCloseLlmModal() {
    setLlmBaseUrl(savedLlmBaseUrl);
    setLlmModelInput(savedLlmModel);
    setLlmSaveNotice("");
    setIsLlmModalOpen(false);
  }

  function handleSaveCameraSelection() {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(cameraPreferenceStorageKey, selectedCameraId);
    }
    setSavedCameraId(selectedCameraId);
    const message = selectedCameraId
      ? tr(`Saved camera preference: ${selectedCameraLabel}.`, `Bevorzugte Kamera gespeichert: ${selectedCameraLabel}.`)
      : tr("Camera preference cleared.", "Kameraeinstellung zurückgesetzt.");
    setCameraMessage(message);
    setCameraSaveNotice(message);
    window.setTimeout(() => {
      handleCloseCameraModal();
    }, 500);
  }

  function updateDiceDraft(index: number, patch: Partial<DiceDetection>) {
    setDiceFrameDraft((current) =>
      current.map((die, currentIndex) => (currentIndex === index ? { ...die, ...patch } : die))
    );
  }

  function addDiceDraftRow() {
    setDiceFrameDraft((current) => [...current, { type: "d20", value: 1 }]);
  }

  function removeDiceDraftRow(index: number) {
    setDiceFrameDraft((current) => current.filter((_, currentIndex) => currentIndex !== index));
  }

  async function handleCaptureDiceFrame() {
    if (cameraStatus !== "ready") {
      setDiceStabilityStatus("error");
      setDiceTestMessage(tr("Start the camera test before capturing a dice frame.", "Starte den Kameratest, bevor du ein Würfelbild aufnimmst."));
      return;
    }

    setDiceStabilityStatus("stabilizing");
    setIsCapturingDice(true);
    setDiceTestMessage(tr("Capturing multiple frames and checking for a stable dice result...", "Mehrere Bilder werden aufgenommen und auf ein stabiles Würfelergebnis geprüft …"));
    try {
      if (showDiceDebugInput && diceFrameDraft.length > 0) {
        const nextFrame: DiceDetectionFrame = {
          frame_id: createClientId("dice-frame"),
          dice: diceFrameDraft,
          confidence: 0.95,
          timestamp: new Date().toISOString(),
        };
        const frames = [...diceFrameHistory, nextFrame].slice(-3);
        setDiceFrameHistory(frames);
        const stabilized = await stabilizeDiceFrames({
          frames,
          min_consensus: 3,
        });

        if (!stabilized.stable) {
          setDetectedDice(diceFrameDraft);
          setDetectedDiceCount(diceFrameDraft.length);
          setEditableDetectedDice(diceFrameDraft.map((die) => die.value));
          setDiceStabilityStatus("stabilizing");
          setDiceTestMessage(tr(`Debug dice recognized. Stabilizing: ${stabilized.matching_frames}/${stabilized.required_matches}.`, `Debug-Würfel erkannt. Stabilisierung: ${stabilized.matching_frames}/${stabilized.required_matches}.`));
          return;
        }

        setDetectedDice(stabilized.stable_dice);
        setDetectedDiceCount(stabilized.stable_dice.length);
        setEditableDetectedDice(stabilized.stable_dice.map((die) => die.value));
        setDiceStabilityStatus("stable");
        setDiceTestMessage(tr(`Stable debug result: ${stabilized.stable_dice.map((die) => `${die.type} shows ${die.value}`).join(", ")}.`, `Stabiles Debug-Ergebnis: ${stabilized.stable_dice.map((die) => `${die.type} zeigt ${die.value}`).join(", ")}.`));
        return;
      }

      const capturedFrames: DiceDetectionFrame[] = [];
      const detectionNotes: string[] = [];
      const detectionImages: string[] = [];
      const detectionBoxes: DiceBox[][] = [];
      for (let index = 0; index < 3; index += 1) {
        const imageDataURL = captureCurrentFrame();
        if (!imageDataURL) {
          throw new Error(tr("No live frame available yet. Wait for the preview and try again.", "Noch kein Live-Bild verfügbar. Warte auf die Vorschau und versuche es erneut."));
        }

        const detection = await detectDiceFromImage({
          image_data_url: imageDataURL,
          language: locale,
        });
        detectionNotes.push(detection.notes);
        detectionImages.push(imageDataURL);
        detectionBoxes.push(detection.boxes);
        capturedFrames.push({
          frame_id: createClientId("dice-frame"),
          dice: detection.dice,
          confidence: detection.confidence || 0.8,
          timestamp: new Date().toISOString(),
        });
        if (index < 2) {
          await sleep(220);
        }
      }

      const nonEmptyFrames = capturedFrames.filter((frame) => frame.dice.length > 0);
      if (nonEmptyFrames.length === 0) {
        setDetectedDice([]);
        setDetectedDiceCount(0);
        const bestIndex = detectionBoxes.reduce((best, current, index, all) => {
          return current.length > all[best].length ? index : best;
        }, 0);
        setDetectedDiceBoxes(detectionBoxes[bestIndex] ?? []);
        setDiceAnalysisImage(detectionImages[bestIndex] ?? null);
        setDiceAnalysisSize(captureCanvasRef.current ? { width: captureCanvasRef.current.width, height: captureCanvasRef.current.height } : null);
        setDiceStabilityStatus("stabilizing");
        const fallbackNote = detectionNotes.find((note) => note && note.trim().length > 0);
        setDiceTestMessage(fallbackNote || tr("No clear dice were recognized across the captured frames.", "In den aufgenommenen Bildern wurden keine Würfel eindeutig erkannt."));
        return;
      }

      const grouped = new Map<
        string,
        {
          matches: number;
          best: DiceDetectionFrame;
          totalConfidence: number;
          bestCount: number;
        }
      >();
      const representativeCountBySignature = new Map<string, number>();
      for (const frame of nonEmptyFrames) {
        const signature = diceSignature(frame.dice);
        const frameCount = capturedFrames.find((candidate) => candidate.frame_id === frame.frame_id)?.dice.length ?? frame.dice.length;
        const current = grouped.get(signature);
        if (!current) {
          grouped.set(signature, {
            matches: 1,
            best: frame,
            totalConfidence: frame.confidence,
            bestCount: frameCount,
          });
          representativeCountBySignature.set(signature, frameCount);
          continue;
        }
        current.matches += 1;
        current.totalConfidence += frame.confidence;
        if (frame.confidence > current.best.confidence) {
          current.best = frame;
          current.bestCount = frameCount;
        }
      }

      const representativeGroup =
        [...grouped.values()].sort((left, right) => {
          if (left.matches === right.matches) {
            return right.totalConfidence - left.totalConfidence;
          }
          return right.matches - left.matches;
        })[0];
      const representative = representativeGroup?.best ?? nonEmptyFrames[nonEmptyFrames.length - 1];
      const representativeCount = representativeGroup?.bestCount ?? representative.dice.length;
      const representativeDetectionIndex = capturedFrames.findIndex((frame) => frame.frame_id === representative.frame_id);
      setDetectedDiceBoxes(detectionBoxes[representativeDetectionIndex] ?? []);
      setDiceAnalysisImage(detectionImages[representativeDetectionIndex] ?? null);
      setDiceAnalysisSize(captureCanvasRef.current ? { width: captureCanvasRef.current.width, height: captureCanvasRef.current.height } : null);

      const frames = [...diceFrameHistory, representative].slice(-3);
      setDiceFrameHistory(frames);
      const stabilized = await stabilizeDiceFrames({
        frames,
        min_consensus: 2,
      });

      if (!stabilized.stable) {
        setDetectedDice(representative.dice ?? []);
        setDetectedDiceCount(representativeCount);
        setEditableDetectedDice((representative.dice ?? []).map((die) => die.value));
        setDiceStabilityStatus("stabilizing");
        setDiceTestMessage(
          tr(`Dice were seen, but not yet stable enough. Matching frames: ${stabilized.matching_frames}/${stabilized.required_matches}. Keep the dice still and capture again.`, `Würfel wurden erkannt, sind aber noch nicht stabil genug. Übereinstimmende Bilder: ${stabilized.matching_frames}/${stabilized.required_matches}. Halte die Würfel still und nimm erneut auf.`)
        );
        return;
      }

      const stableDice = stabilized.stable_dice ?? [];
      const nextDice =
        activeGuidedRollStep
          ? stableDice.filter((die) => die.type === activeGuidedRollStep.type).slice(0, activeGuidedRollStep.count)
          : stableDice;
      const filledValues = activeGuidedRollStep
        ? Array.from({ length: activeGuidedRollStep.count }, (_, index) => nextDice[index]?.value ?? 0)
        : nextDice.map((die) => die.value);

      setDetectedDice(
        activeGuidedRollStep
          ? filledValues.map((value) => ({ type: activeGuidedRollStep.type, value }))
          : nextDice
      );
      setDetectedDiceCount(activeGuidedRollStep ? representativeCount : representativeCount);
      setEditableDetectedDice(filledValues);
      setDiceStabilityStatus("stable");
      if (activeGuidedRollStep) {
        const matched = nextDice.length;
        setDiceTestMessage(
          matched === activeGuidedRollStep.count
            ? tr(`Step detected: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} for ${activeGuidedRollStep.label}. Check and confirm the values.`, `Schritt erkannt: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} für ${activeGuidedRollStep.label}. Prüfe und bestätige die Werte.`)
            : tr(`Partially detected: expected ${activeGuidedRollStep.count}${activeGuidedRollStep.type}, read ${matched}. Check or correct the values.`, `Teilweise erkannt: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} erwartet, ${matched} gelesen. Bitte Werte prüfen oder korrigieren.`)
        );
      } else {
        setDiceTestMessage(tr(`Roll detected: ${stableDice.map((die) => `${die.type} shows ${die.value}`).join(", ")}.`, `Wurf erkannt: ${stableDice.map((die) => `${die.type} zeigt ${die.value}`).join(", ")}.`));
      }
    } catch (error) {
      setDiceStabilityStatus("error");
      setDiceTestMessage(error instanceof Error ? error.message : tr("Dice test failed.", "Würfeltest fehlgeschlagen."));
    } finally {
      setIsCapturingDice(false);
    }
  }

  function startGuidedDiceTest() {
    const plan = generateGuidedRollPlan(locale);
    resetDiceTest();
    setIsDiceTestActive(true);
    setGuidedRollPlan(plan);
    const first = plan[0];
    setDiceTestMessage(tr(`Step 1/${plan.length}: Roll ${first.count}${first.type} now for ${first.label}.`, `Schritt 1/${plan.length}: Würfle jetzt ${first.count}${first.type} für ${first.label}.`));
  }

  function updateEditableDetectedDie(index: number, value: number) {
    setEditableDetectedDice((current) => current.map((entry, currentIndex) => (currentIndex === index ? value : entry)));
    setDetectedDice((current) =>
      current.map((entry, currentIndex) => (currentIndex === index ? { ...entry, value } : entry))
    );
  }

  function confirmGuidedRollStep() {
    if (!activeGuidedRollStep) {
      return;
    }
    if (editableDetectedDice.length !== activeGuidedRollStep.count || editableDetectedDice.some((value) => value <= 0)) {
      setDiceStabilityStatus("error");
      setDiceTestMessage(tr(`Enter all values for ${activeGuidedRollStep.count}${activeGuidedRollStep.type} before confirming.`, `Trage alle Werte für ${activeGuidedRollStep.count}${activeGuidedRollStep.type} ein, bevor du bestätigst.`));
      return;
    }

    const nextPlan = guidedRollPlan.map((step, index) => {
      if (step.id === activeGuidedRollStep.id) {
        return { ...step, status: "confirmed" as const, confirmedValues: editableDetectedDice };
      }
      const activeIndex = guidedRollPlan.findIndex((item) => item.id === activeGuidedRollStep.id);
      if (index === activeIndex + 1 && step.status === "pending") {
        return { ...step, status: "active" as const };
      }
      return step;
    });
    setGuidedRollPlan(nextPlan);

    const nextActive = nextPlan.find((step) => step.status === "active");
    if (nextActive) {
      setDetectedDice([]);
      setDetectedDiceCount(0);
      setEditableDetectedDice([]);
      setDetectedDiceBoxes([]);
      setDiceAnalysisImage(null);
      setDiceAnalysisSize(null);
      setDiceFrameHistory([]);
      setDiceStabilityStatus("idle");
      setDiceTestMessage(tr(`Next step: Roll ${nextActive.count}${nextActive.type} now for ${nextActive.label}.`, `Nächster Schritt: Würfle jetzt ${nextActive.count}${nextActive.type} für ${nextActive.label}.`));
      return;
    }

    setDiceStabilityStatus("stable");
    setDiceTestMessage(tr("Guided dice test complete. All requested roll steps were confirmed.", "Geführter Würfeltest abgeschlossen. Alle angeforderten Würfe wurden bestätigt."));
  }

  function handleSaveAudioSelection() {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(microphonePreferenceStorageKey, selectedMicrophoneId);
      window.localStorage.setItem(speakerPreferenceStorageKey, selectedSpeakerId);
    }
    setSavedMicrophoneId(selectedMicrophoneId);
    setSavedSpeakerId(selectedSpeakerId);
    const message = tr(`Saved audio settings: ${selectedMicrophoneLabel} / ${selectedSpeakerLabel}.`, `Audioeinstellungen gespeichert: ${selectedMicrophoneLabel} / ${selectedSpeakerLabel}.`);
    setAudioMessage(message);
    setAudioSaveNotice(message);
    window.setTimeout(() => {
      handleCloseAudioModal();
    }, 500);
  }

  function handleSaveLlmSettings() {
    setLlmSaveNotice("");
    setLlmTestStatus("idle");
    void (async () => {
      try {
        const saved = await updateSystemConfig({
          llm_base_url: llmBaseUrl.trim(),
          llm_model: llmModelInput.trim(),
        });
        setSavedLlmBaseUrl(saved.llm_base_url);
        setLlmBaseUrl(saved.llm_base_url);
        setSavedLlmModel(saved.llm_model);
        setLlmModelInput(saved.llm_model);
        const message = tr(`LLM active: ${saved.llm_model} @ ${saved.llm_base_url}`, `LLM aktiv: ${saved.llm_model} @ ${saved.llm_base_url}`);
        setLlmSaveNotice(message);
        setLlmTestMessage(message);
        window.setTimeout(() => {
          handleCloseLlmModal();
          window.location.reload();
        }, 500);
      } catch (error) {
        setLlmSaveNotice(error instanceof Error ? error.message : tr("Could not save LLM settings.", "LLM-Einstellungen konnten nicht gespeichert werden."));
      }
    })();
  }

  async function handleFetchModels() {
    if (!llmBaseUrl.trim()) {
      setLlmTestStatus("error");
      setLlmTestMessage(tr("Enter a base URL before fetching models.", "Gib eine Basis-URL ein, bevor du Modelle abrufst."));
      return;
    }
    setLlmTestStatus("running");
    setLlmTestMessage(tr("Fetching available models...", "Verfügbare Modelle werden abgerufen …"));
    try {
      const data = await fetchLLMModels({ llm_base_url: llmBaseUrl.trim(), llm_model: llmModelInput.trim() });
      const models = data.models;
      setAvailableModels(models);
      if (!llmModelInput && models[0]) {
        setLlmModelInput(models[0]);
      }
      setLlmTestStatus("success");
      setLlmTestMessage(models.length > 0 ? tr(`${models.length} models loaded.`, `${models.length} Modelle geladen.`) : tr("Model endpoint responded, but no models were listed.", "Der Modell-Endpunkt antwortete, listete aber keine Modelle auf."));
    } catch (error) {
      setLlmTestStatus("error");
      setLlmTestMessage(error instanceof Error ? error.message : tr("Fetching models failed.", "Modelle konnten nicht abgerufen werden."));
    }
  }

  async function handleLlmConnectionTest() {
    if (!llmBaseUrl.trim() || !llmModelInput.trim()) {
      setLlmTestStatus("error");
      setLlmTestMessage(tr("Enter a base URL and choose a model before running the DM test.", "Gib eine Basis-URL ein und wähle ein Modell, bevor du den Spielleiter-Test startest."));
      return;
    }

    setLlmTestStatus("running");
    setLlmTestMessage(tr("Sending a short GM prompt through the secure API backend...", "Ein kurzer Spielleiter-Prompt wird über das sichere API-Backend gesendet …"));
    try {
      const data = await testLLMConnection({ llm_base_url: llmBaseUrl.trim(), llm_model: llmModelInput.trim() });
      const content = data.content.trim();
      setLlmTestStatus("success");
      setLlmTestMessage(content || tr(`${data.model || llmModelInput} responded successfully.`, `${data.model || llmModelInput} hat erfolgreich geantwortet.`));
    } catch (error) {
      setLlmTestStatus("error");
      setLlmTestMessage(error instanceof Error ? error.message : tr("LLM test failed.", "LLM-Test fehlgeschlagen."));
    }
  }

  function handlePlayerScreenTest() {
    if (typeof window !== "undefined") {
      window.open("/player-screen", "_blank", "noopener,noreferrer");
    }
    setPlayerScreenTestStatus("success");
    setPlayerScreenTestMessage(tr("Player screen opened in a new tab. Verify the display or projector output.", "Die Spieleransicht wurde in einem neuen Tab geöffnet. Prüfe die Anzeige oder Projektorausgabe."));
  }

  async function handleStartFungalCavernsDemo() {
    setDemoStatus("creating");
    setDemoError("");
    try {
      const demo = await createFungalCavernsDemo(locale);
      window.location.assign(demo.gm_url);
    } catch (error) {
      setDemoStatus("error");
      setDemoError(error instanceof Error ? error.message : tr("The demo could not be created.", "Die Demo konnte nicht erstellt werden."));
    }
  }

  const cameraTone: "default" | "ready" | "warning" | "live" | "info" =
    cameraStatus === "ready" || cameraConfigured ? "ready" : cameraStatus === "unsupported" || cameraStatus === "error" ? "warning" : "info";
  const cameraDetail =
    cameraStatus === "unsupported"
      ? tr("Browser security context is blocking camera APIs", "Der Sicherheitskontext des Browsers blockiert Kamera-APIs")
      : cameraConfigured
        ? tr(`Configured: ${savedCameraLabel}`, `Konfiguriert: ${savedCameraLabel}`)
        : tr("Open settings to choose and test a camera", "Öffne die Einstellungen, um eine Kamera auszuwählen und zu testen");
  const audioTone: "default" | "ready" | "warning" | "live" | "info" =
    audioStatus === "ready" || audioConfigured ? "ready" : audioStatus === "unsupported" || audioStatus === "error" ? "warning" : "info";
  const audioDetail =
    audioStatus === "unsupported"
      ? tr("Browser security context is blocking audio APIs", "Der Sicherheitskontext des Browsers blockiert Audio-APIs")
		: `${stt.provider || "audio"}: ${stt.model || "STT"} → ${tts.model || "TTS"}${audioConfigured ? ` · ${savedMicrophoneLabel} / ${savedSpeakerLabel}` : tr(" · choose browser devices", " · Browsergeräte auswählen")}`;
  const llmConfigured = savedLlmBaseUrl.trim().length > 0 && savedLlmModel.trim().length > 0;
  const llmTone: "default" | "ready" | "warning" | "live" | "info" =
    llmTestStatus === "success" || llmConfigured ? "ready" : llmTestStatus === "error" ? "warning" : "info";
  const llmDetail =
    llmConfigured
      ? `${savedLlmModel} @ ${savedLlmBaseUrl}`
      : tr("Open settings to configure model routing", "Öffne die Einstellungen, um das Modell-Routing zu konfigurieren");

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow={tr("Control Center", "Kontrollzentrum")}
        title={tr("Session readiness before the AI takes over", "Sitzungsbereitschaft, bevor die KI übernimmt")}
        description={tr("Check devices, model access, player outputs, and live readiness before starting a session.", "Prüfe Geräte, Modellzugriff, Spielerausgaben und Live-Bereitschaft, bevor du eine Sitzung startest.")}
        actions={
          <div className="button-row">
            <button className="studio-button" disabled={demoStatus === "creating"} onClick={() => void handleStartFungalCavernsDemo()} type="button">
              {demoStatus === "creating" ? tr("Preparing demo…", "Demo wird vorbereitet …") : tr("Start Fungal Caverns Demo", "Demo „Fungal Caverns“ starten")}
            </button>
            <Link className="studio-button studio-button--ghost" href="/player-screen">
              {tr("Player Screen", "Spieleransicht")}
            </Link>
            <Link className="studio-button studio-button--ghost" href={liveSession ? `/sessions/${liveSession.id}` : "/sessions"}>
              {tr("Open Live Session", "Live-Sitzung öffnen")}
            </Link>
            {demoError ? <span className="error-copy">{demoError}</span> : null}
          </div>
        }
      />

      <section className="hero-grid">
        <Panel
          title={tr("System Health", "Systemzustand")}
          description={tr("Confirm that the model, database, player outputs, and devices are ready.", "Prüfe, ob Modell, Datenbank, Spielerausgaben und Geräte bereit sind.")}
          className="hero-panel"
          action={<StatusPill tone={readyTone}>{readyTone === "ready" ? tr("Ready", "Bereit") : tr("Needs Setup", "Einrichtung nötig")}</StatusPill>}
        >
          <div className="status-grid">
            <button className="status-card status-card--interactive" onClick={handleOpenCameraModal} type="button">
              <div className="status-card__icon">
                <Camera size={18} />
              </div>
              <div>
                <div className="status-card__head">
                  <strong>{tr("Camera & Dice", "Kamera & Würfel")}</strong>
                  <StatusPill tone={cameraTone}>{cameraConfigured ? tr("Ready", "Bereit") : tr("Setup", "Einrichten")}</StatusPill>
                </div>
                <p>{cameraDetail}</p>
              </div>
            </button>
            <button className="status-card status-card--interactive" onClick={handleOpenAudioModal} type="button">
              <div className="status-card__icon">
                <Volume2 size={18} />
              </div>
              <div>
                <div className="status-card__head">
				  <strong>Audio · {stt.model || "STT"} / {tts.model || "TTS"}</strong>
                  <StatusPill tone={audioTone}>{audioConfigured ? tr("Ready", "Bereit") : tr("Setup", "Einrichten")}</StatusPill>
                </div>
                <p>{audioDetail}</p>
              </div>
            </button>
            <button className="status-card status-card--interactive" onClick={handleOpenLlmModal} type="button">
              <div className="status-card__icon">
                <Brain size={18} />
              </div>
              <div>
                <div className="status-card__head">
                  <strong>{tr("AI Model", "KI-Modell")} · Powered by {savedLlmModel || "GPT-5.6"}</strong>
                  <StatusPill tone={llmTone}>{llmConfigured ? tr("Ready", "Bereit") : tr("Setup", "Einrichten")}</StatusPill>
                </div>
                <p>{llmDetail}</p>
              </div>
            </button>
            {checks.map((item) => {
              const Icon = item.icon;
              return (
                <article className="status-card" key={item.name}>
                  <div className="status-card__icon">
                    <Icon size={18} />
                  </div>
                  <div>
                    <div className="status-card__head">
                      <strong>{item.name}</strong>
                      <StatusPill tone={item.tone}>{tr("Ready", "Bereit")}</StatusPill>
                    </div>
                    <p>{item.detail}</p>
                  </div>
                </article>
              );
            })}
          </div>
        </Panel>
      </section>

      <section className="dashboard-grid">
        <Panel title={tr("Live Summary", "Live-Zusammenfassung")} description={tr("Current content inventory and system footprint.", "Aktueller Inhaltsbestand und Systemumfang.")}>
          <div className="stat-grid">
            <StatCard label={tr("Online services", "Online-Dienste")} value={`${onlineServices}/${services.length}`} detail={tr("API and database verified", "API und Datenbank geprüft")} />
            <StatCard label={tr("Campaigns", "Kampagnen")} value={counts.campaigns ?? 0} />
            <StatCard label={tr("Sessions", "Sitzungen")} value={counts.sessions ?? 0} />
            <StatCard label={tr("Documents", "Dokumente")} value={counts.documents ?? 0} />
            <StatCard label={tr("Assets", "Medien")} value={counts.assets ?? 0} />
            <StatCard label={tr("Characters", "Charaktere")} value={counts.characters ?? 0} />
            <StatCard label={tr("Chunks", "Abschnitte")} value={counts.document_chunks ?? 0} />
          </div>
        </Panel>

        {llmGateway ? (
          <Panel title="LLM Gateway" description={tr("Runtime status for concurrency, circuit breaker, and archived sessions.", "Laufzeitstatus für Parallelität, Schutzschaltung und archivierte Sitzungen.")}>
            <div className="stat-grid">
              <StatCard label={tr("Status", "Status")} value={llmGateway.status} />
              <StatCard label={tr("In Flight", "In Bearbeitung")} value={llmGateway.in_flight} />
              <StatCard label={tr("Max Concurrent", "Maximal parallel")} value={llmGateway.max_concurrent_requests} />
              <StatCard label={tr("Rejected", "Abgelehnt")} value={llmGateway.rejected_requests} />
              <StatCard label={tr("Timeouts", "Zeitüberschreitungen")} value={llmGateway.timeout_count} />
              <StatCard label={tr("Active Sessions", "Aktive Sitzungen")} value={llmGateway.active_gateway_sessions} />
              <StatCard label={tr("Archived Sessions", "Archivierte Sitzungen")} value={llmGateway.archived_gateway_sessions} />
            </div>
            <div className="meta-chip-row">
              <StatusPill tone={llmGateway.circuit_breaker_open ? "warning" : "ready"}>
                {llmGateway.circuit_breaker_open ? tr("Circuit Open", "Schutzschaltung offen") : tr("Circuit Closed", "Schutzschaltung geschlossen")}
              </StatusPill>
              <StatusPill tone="default">{tr("Queue", "Warteschlange")} {llmGateway.queue_length}</StatusPill>
              <StatusPill tone="default">{tr("Failures", "Fehler")} {llmGateway.consecutive_failures}</StatusPill>
            </div>
            {llmGateway.last_error ? <p className="muted-copy">{llmGateway.last_error}</p> : null}
            <div className="builder-documents-grid">
              {llmGateway.profiles.map((profile) => (
                <article className="builder-document-card" key={profile.name}>
                  <strong>{profile.name}</strong>
                  <p>{tr("Input", "Eingabe")} {profile.max_input_tokens} · {tr("Output", "Ausgabe")} {profile.max_output_tokens}</p>
                  <p>{tr("Timeout", "Zeitlimit")} {profile.timeout_seconds}s · {tr("Window", "Fenster")} {profile.live_turn_window}</p>
                </article>
              ))}
            </div>
          </Panel>
        ) : null}

        <Panel title={tr("Session Readiness", "Sitzungsbereitschaft")} description={tr("Requirements for a live AI-led session.", "Voraussetzungen für eine live von der KI geleitete Sitzung.")}>
          <div className="list-stack">
            <article className="list-row">
              <div className="list-row__icon">
                <PlayCircle size={18} />
              </div>
              <div className="list-row__body">
                <strong>{liveSession ? liveSession.current_scene || tr("Session selected", "Sitzung ausgewählt") : tr("No session selected", "Keine Sitzung ausgewählt")}</strong>
                <p>{liveSession ? `${liveSession.current_location || tr("No location", "Kein Ort")} · ${liveSession.status}` : tr("Create or start a session first.", "Erstelle oder starte zuerst eine Sitzung.")}</p>
              </div>
              <StatusPill tone={liveSession ? "live" : "warning"}>{statusLabel(liveSession ? liveSession.status : "missing")}</StatusPill>
            </article>
            <article className="list-row">
              <div className="list-row__icon">
                <Wifi size={18} />
              </div>
              <div className="list-row__body">
                <strong>{tr("Player portal join state", "Beitrittsstatus im Spielerportal")}</strong>
                <p>{joinedPlayers} {tr("joined players", "beigetretene Spieler")} · {playerLinks.length} {tr("invite slots", "Einladungsplätze")}</p>
              </div>
              <StatusPill tone={playerLinks.length > 0 ? "ready" : "warning"}>{playerLinks.length > 0 ? tr("links ready", "Links bereit") : tr("no links", "keine Links")}</StatusPill>
            </article>
            <article className="list-row">
              <div className="list-row__icon">
                <Radio size={18} />
              </div>
              <div className="list-row__body">
                <strong>{tr("Content and model context", "Inhalts- und Modellkontext")}</strong>
                <p>{counts.documents ?? 0} {tr("documents", "Dokumente")}, {counts.assets ?? 0} {tr("assets", "Medien")}, {tr("model", "Modell")} {llm.model ? tr("configured", "konfiguriert") : tr("missing", "fehlt")}</p>
              </div>
              <StatusPill tone={llm.model && (counts.documents ?? 0) > 0 ? "ready" : "warning"}>
                {llm.model && (counts.documents ?? 0) > 0 ? "ready" : "incomplete"}
              </StatusPill>
            </article>
          </div>
        </Panel>
      </section>

      {isCameraModalOpen ? (
        <div className="modal-overlay" onClick={handleCloseCameraModal} role="presentation">
          <section
            aria-labelledby="camera-settings-title"
            aria-modal="true"
            className="modal-card modal-card--camera"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">{tr("Camera Settings", "Kameraeinstellungen")}</p>
                <h2 className="studio-panel__title" id="camera-settings-title">
                  {tr("Configure camera and dice capture input", "Kamera und Würfelerfassung konfigurieren")}
                </h2>
                <p className="studio-panel__description">{tr("Choose a camera, test the live preview, and save it for future sessions.", "Wähle eine Kamera, teste die Live-Vorschau und speichere sie für zukünftige Sitzungen.")}</p>
              </div>
              <button className="icon-button" aria-label={tr("Close camera settings", "Kameraeinstellungen schließen")} onClick={handleCloseCameraModal} type="button">
                <X size={18} />
              </button>
            </div>

            <div className="camera-device-panel">
              <div className="form-grid">
                <select onChange={(event) => setSelectedCameraId(event.target.value)} value={selectedCameraId}>
                  <option value="">{cameraDevices.length === 0 ? tr("No camera devices detected", "Keine Kameras erkannt") : tr("Default browser camera", "Standard-Browserkamera")}</option>
                  {cameraDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.label || `${tr("Camera", "Kamera")} ${device.deviceId.slice(0, 6)}`}
                    </option>
                  ))}
                </select>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void refreshCameraDevices()} type="button">
                    {tr("Detect Cameras", "Kameras erkennen")}
                  </button>
                  <button className="studio-button" disabled={isTestingCamera} onClick={() => void handleCameraTest()} type="button">
                    {isTestingCamera ? tr("Testing...", "Test läuft …") : tr("Start Camera Test", "Kameratest starten")}
                  </button>
                  <button
                    className="studio-button studio-button--ghost"
                    disabled={cameraStatus !== "ready"}
                    onClick={handleStopCameraTest}
                    type="button"
                  >
                    <Square size={16} />
                    {tr("Stop Test", "Test beenden")}
                  </button>
                </div>
                <div className="list-stack">
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Camera size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("Camera status", "Kamerastatus")}</strong>
                      <p>{cameraMessage}</p>
                    </div>
                    <StatusPill tone={cameraStatus === "ready" ? "ready" : cameraStatus === "unsupported" ? "warning" : "info"}>
                      {statusLabel(cameraStatus)}
                    </StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Radio size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("Saved preference", "Gespeicherte Auswahl")}</strong>
                      <p>{cameraConfigured ? savedCameraLabel : tr("No camera saved yet.", "Noch keine Kamera gespeichert.")}</p>
                    </div>
                    <StatusPill tone={cameraConfigured ? "ready" : "warning"}>{statusLabel(cameraConfigured ? "saved" : "missing")}</StatusPill>
                  </article>
                </div>
                <div className="hint-box">
                  <strong>{tr("Hint:", "Hinweis:")}</strong> {tr("Camera APIs usually work best on HTTPS or localhost. Stop the test to release the device.", "Kamera-APIs funktionieren meist am besten über HTTPS oder localhost. Beende den Test, um das Gerät freizugeben.")}
                </div>
                <div className="hint-box">
                  <strong>{tr("Dice Test:", "Würfeltest:")}</strong> {tr("The current version is tuned for physical ", "Die aktuelle Version ist auf physische ")}<code>d6</code>{tr(" dice. Start the camera, place the dice in frame, and capture them.", "-Würfel abgestimmt. Starte die Kamera, lege die Würfel ins Bild und nimm sie auf.")}
                </div>
                {guidedRollPlan.length > 0 ? (
                  <div className="list-stack">
                    {guidedRollPlan.map((step, index) => (
                      <article className="list-row" key={step.id}>
                        <div className="list-row__icon">
                          <Dices size={18} />
                        </div>
                        <div className="list-row__body">
                          <strong>
                            {tr("Step", "Schritt")} {index + 1}: {step.count}
                            {step.type}
                          </strong>
                          <p>
                            {step.label}
                            {step.confirmedValues.length > 0 ? ` • ${tr("confirmed", "bestätigt")}: ${step.confirmedValues.join(", ")}` : ""}
                          </p>
                        </div>
                        <StatusPill tone={step.status === "confirmed" ? "ready" : step.status === "active" ? "info" : "warning"}>
                          {statusLabel(step.status)}
                        </StatusPill>
                      </article>
                    ))}
                  </div>
                ) : null}
              </div>
              <div className="camera-box camera-box--compact">
                <video autoPlay className="camera-preview" muted playsInline ref={videoRef} />
                <canvas hidden ref={captureCanvasRef} />
                {cameraStatus === "ready" ? (
                  <div className="camera-preview__badge">
                    <StatusPill tone="ready">{tr("Live Preview", "Live-Vorschau")}</StatusPill>
                  </div>
                ) : (
                  <div className="camera-placeholder">
                    <Camera size={28} />
                    <span>{cameraStatus === "unsupported" ? tr("Camera unsupported", "Kamera nicht unterstützt") : tr("No live preview yet", "Noch keine Live-Vorschau")}</span>
                  </div>
                )}
              </div>
            </div>

            <div className="list-stack">
              <article className="list-row">
                <div className="list-row__icon">
                  <Dices size={18} />
                </div>
                <div className="list-row__body">
                  <strong>{tr("Dice test", "Würfeltest")}</strong>
                  <p>{diceTestMessage}</p>
                </div>
                <StatusPill tone={diceStabilityStatus === "stable" ? "ready" : diceStabilityStatus === "error" ? "warning" : "info"}>
                  {statusLabel(diceStabilityStatus)}
                </StatusPill>
              </article>
            </div>

            {diceAnalysisImage ? (
              <div
                className="camera-box camera-box--analysis"
                style={
                  diceAnalysisSize
                    ? { aspectRatio: `${diceAnalysisSize.width} / ${diceAnalysisSize.height}` }
                    : undefined
                }
              >
                <div className="camera-preview camera-preview--analysis">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img alt={tr("Dice analysis snapshot", "Aufnahme der Würfelanalyse")} src={diceAnalysisImage} />
                  {detectedDiceBoxes.map((box, index) => (
                    <div
                      className="dice-box-overlay"
                      key={`dice-box-${index}`}
                      style={{
                        left: `${((box.x || 0) / Math.max(1, diceAnalysisSize?.width || 1)) * 100}%`,
                        top: `${((box.y || 0) / Math.max(1, diceAnalysisSize?.height || 1)) * 100}%`,
                        width: `${((box.w || 0) / Math.max(1, diceAnalysisSize?.width || 1)) * 100}%`,
                        height: `${((box.h || 0) / Math.max(1, diceAnalysisSize?.height || 1)) * 100}%`,
                      }}
                    />
                  ))}
                </div>
                <div className="camera-preview__badge">
                  <StatusPill tone="info">{detectedDiceBoxes.length} {tr("boxes", "Rahmen")}</StatusPill>
                </div>
              </div>
            ) : null}

            <div className="button-row">
              <button
                className="studio-button studio-button--ghost"
                onClick={() => {
                  startGuidedDiceTest();
                }}
                type="button"
              >
                {tr("Start Guided Dice Test", "Geführten Würfeltest starten")}
              </button>
              <button className="studio-button studio-button--ghost" disabled={!isDiceTestActive} onClick={resetDiceTest} type="button">
                <RefreshCw size={16} />
                {tr("Reset Dice Test", "Würfeltest zurücksetzen")}
              </button>
              <button className="studio-button" disabled={!isDiceTestActive || cameraStatus !== "ready" || !activeGuidedRollStep} onClick={() => void handleCaptureDiceFrame()} type="button">
                {tr("Capture Dice Frame", "Würfelbild aufnehmen")}
              </button>
              <button
                className="studio-button studio-button--ghost"
                disabled={!activeGuidedRollStep || editableDetectedDice.length === 0}
                onClick={confirmGuidedRollStep}
                type="button"
              >
                {tr("Confirm Step", "Schritt bestätigen")}
              </button>
            </div>

            <div className="list-stack">
              <article className="list-row">
                <div className="list-row__icon">
                  <Dices size={18} />
                </div>
                <div className="list-row__body">
                  <strong>{tr("Detected result", "Erkanntes Ergebnis")}</strong>
                  <p>
                    {activeGuidedRollStep
                      ? tr(`Expected: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} for ${activeGuidedRollStep.label}.`, `Erwartet: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} für ${activeGuidedRollStep.label}.`)
                      : detectedDice.length > 0
                      ? tr(`${detectedDiceCount || detectedDice.length} dice found. Read values: ${detectedDice.map((die) => `${die.type} shows ${die.value}`).join(", ")}`, `${detectedDiceCount || detectedDice.length} Würfel gefunden. Gelesene Werte: ${detectedDice.map((die) => `${die.type} zeigt ${die.value}`).join(", ")}`)
                      : tr("No stable dice result yet.", "Noch kein stabiles Würfelergebnis.")}
                  </p>
                </div>
                <StatusPill tone={detectedDice.length > 0 && diceStabilityStatus === "stable" ? "ready" : "info"}>
                  {detectedDiceCount > 0 ? `${detectedDiceCount} found` : detectedDice.length > 0 ? `${detectedDice.length} read` : "waiting"}
                </StatusPill>
              </article>
            </div>

            {activeGuidedRollStep && editableDetectedDice.length > 0 ? (
              <div className="ability-grid">
                {editableDetectedDice.map((value, index) => (
                  <article className="ability-card" key={`guided-die-${index}`}>
                    <strong>
                      {activeGuidedRollStep.type} #{index + 1}
                    </strong>
                    <input
                      max={diceTypeMaxValue(activeGuidedRollStep.type)}
                      min={activeGuidedRollStep.type === "d100" ? 10 : 1}
                      onChange={(event) => updateEditableDetectedDie(index, Number(event.target.value) || 0)}
                      step={activeGuidedRollStep.type === "d100" ? 10 : 1}
                      type="number"
                      value={value}
                    />
                  </article>
                ))}
              </div>
            ) : detectedDice.length > 0 ? (
              <div className="meta-chip-row">
                {detectedDice.map((die, index) => (
                  <StatusPill key={`detected-${index}`} tone={diceStabilityStatus === "stable" ? "ready" : "info"}>
                    {die.type} = {die.value}
                  </StatusPill>
                ))}
              </div>
            ) : null}

            <div className="button-row">
              <button className="studio-button studio-button--ghost" onClick={() => setShowDiceDebugInput((current) => !current)} type="button">
                {showDiceDebugInput ? tr("Hide Debug Input", "Debug-Eingabe ausblenden") : tr("Open Debug Input", "Debug-Eingabe öffnen")}
              </button>
            </div>

            {showDiceDebugInput ? (
              <div className="page-stack">
                <div className="hint-box">
                  <strong>{tr("Debug Input:", "Debug-Eingabe:")}</strong> {tr("Use this only to test backend stabilization.", "Nur zum Testen der Backend-Stabilisierung verwenden.")}
                </div>
                <div className="ability-grid">
                  {diceFrameDraft.map((die, index) => (
                    <article className="ability-card" key={`dice-draft-${index}`}>
                      <select onChange={(event) => updateDiceDraft(index, { type: event.target.value as DiceType })} value={die.type}>
                        <option value="d4">d4</option>
                        <option value="d6">d6</option>
                        <option value="d8">d8</option>
                        <option value="d10">d10</option>
                        <option value="d12">d12</option>
                        <option value="d20">d20</option>
                        <option value="d100">d100</option>
                      </select>
                      <input
                        min={1}
                        onChange={(event) => updateDiceDraft(index, { value: Number(event.target.value) || 1 })}
                        type="number"
                        value={die.value}
                      />
                      <button className="studio-button studio-button--ghost" onClick={() => removeDiceDraftRow(index)} type="button">
                        {tr("Remove", "Entfernen")}
                      </button>
                    </article>
                  ))}
                </div>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={addDiceDraftRow} type="button">
                    {tr("Add Die", "Würfel hinzufügen")}
                  </button>
                </div>
              </div>
            ) : null}

            <div className="modal-card__footer">
              {cameraSaveNotice ? <p className="camera-save-notice">{cameraSaveNotice}</p> : <span className="modal-card__spacer" />}
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={handleCloseCameraModal} type="button">
                  {tr("Cancel", "Abbrechen")}
                </button>
                <button className="studio-button" onClick={handleSaveCameraSelection} type="button">
                  {tr("Save Camera Setting", "Kameraeinstellung speichern")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}

      {isAudioModalOpen ? (
        <div className="modal-overlay" onClick={handleCloseAudioModal} role="presentation">
          <section
            aria-labelledby="audio-settings-title"
            aria-modal="true"
            className="modal-card modal-card--camera"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">{tr("Audio Settings", "Audioeinstellungen")}</p>
                <h2 className="studio-panel__title" id="audio-settings-title">
                  {tr("Configure sound and microphone input", "Tonausgabe und Mikrofoneingang konfigurieren")}
                </h2>
                <p className="studio-panel__description">{tr("Choose and test a microphone and browser output, then save them for future sessions.", "Wähle und teste Mikrofon und Browserausgabe und speichere sie für zukünftige Sitzungen.")}</p>
              </div>
              <button className="icon-button" aria-label={tr("Close audio settings", "Audioeinstellungen schließen")} onClick={handleCloseAudioModal} type="button">
                <X size={18} />
              </button>
            </div>

            <div className="camera-device-panel">
              <div className="form-grid">
                <select onChange={(event) => setSelectedMicrophoneId(event.target.value)} value={selectedMicrophoneId}>
                  <option value="">{microphoneDevices.length === 0 ? tr("No microphones detected", "Keine Mikrofone erkannt") : tr("Default microphone", "Standardmikrofon")}</option>
                  {microphoneDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.label || `${tr("Microphone", "Mikrofon")} ${device.deviceId.slice(0, 6)}`}
                    </option>
                  ))}
                </select>
                <select onChange={(event) => setSelectedSpeakerId(event.target.value)} value={selectedSpeakerId}>
                  <option value="">{speakerDevices.length === 0 ? tr("No speakers detected", "Keine Lautsprecher erkannt") : tr("Default browser output", "Standard-Browserausgabe")}</option>
                  {speakerDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.label || `${tr("Speaker", "Lautsprecher")} ${device.deviceId.slice(0, 6)}`}
                    </option>
                  ))}
                </select>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void refreshAudioDevices()} type="button">
                    {tr("Detect Audio Devices", "Audiogeräte erkennen")}
                  </button>
                  <button className="studio-button" disabled={isTestingMicrophone} onClick={() => void handleMicrophoneTest()} type="button">
                    {isTestingMicrophone ? tr("Testing...", "Test läuft …") : tr("Start Microphone Test", "Mikrofontest starten")}
                  </button>
                  <button className="studio-button studio-button--ghost" onClick={() => void handleSpeakerTest()} type="button">
                    {tr("Play Test Sound", "Testton abspielen")}
                  </button>
                  <button className="studio-button studio-button--ghost" disabled={audioStatus !== "ready"} onClick={stopMicrophoneTest} type="button">
                    <Square size={16} />
                    {tr("Stop Test", "Test beenden")}
                  </button>
                </div>
                <div className="list-stack">
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Mic size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("Audio status", "Audiostatus")}</strong>
                      <p>{audioMessage}</p>
                    </div>
                    <StatusPill tone={audioStatus === "ready" ? "ready" : audioStatus === "unsupported" ? "warning" : "info"}>
                      {audioStatus}
                    </StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Volume2 size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("Saved preference", "Gespeicherte Auswahl")}</strong>
                      <p>{savedMicrophoneLabel} / {savedSpeakerLabel}</p>
                    </div>
                    <StatusPill tone={audioConfigured ? "ready" : "warning"}>{statusLabel(audioConfigured ? "saved" : "missing")}</StatusPill>
                  </article>
                </div>
                <div className="hint-box">
                  <strong>{tr("Hint:", "Hinweis:")}</strong> {tr("Speaker routing depends on browser support. Otherwise the test uses the default output.", "Die Lautsprecherzuordnung hängt vom Browser ab. Andernfalls nutzt der Test die Standardausgabe.")}
                </div>
              </div>
              <div className="audio-meter-card">
                <div className="audio-meter-card__head">
                  <strong>{tr("Microphone level", "Mikrofonpegel")}</strong>
                  <StatusPill tone={audioStatus === "ready" ? "ready" : "info"}>{Math.round(micLevel)}%</StatusPill>
                </div>
                <div className="audio-meter">
                  <div className="audio-meter__fill" style={{ width: `${micLevel}%` }} />
                </div>
                <p className="studio-panel__description">{tr("Use this to check the browser signal before voice capture.", "Prüfe damit das Browsersignal vor einer Sprachaufnahme.")}</p>
              </div>
            </div>

            <div className="modal-card__footer">
              {audioSaveNotice ? <p className="camera-save-notice">{audioSaveNotice}</p> : <span className="modal-card__spacer" />}
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={handleCloseAudioModal} type="button">
                  {tr("Cancel", "Abbrechen")}
                </button>
                <button className="studio-button" onClick={handleSaveAudioSelection} type="button">
                  {tr("Save Audio Settings", "Audioeinstellungen speichern")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}

      {isLlmModalOpen ? (
        <div className="modal-overlay" onClick={handleCloseLlmModal} role="presentation">
          <section
            aria-labelledby="llm-settings-title"
            aria-modal="true"
            className="modal-card modal-card--camera"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="modal-card__header">
              <div>
                <p className="eyebrow">{tr("AI Model Settings", "KI-Modelleinstellungen")}</p>
                <h2 className="studio-panel__title" id="llm-settings-title">
                  {tr("Configure AI model routing", "KI-Modell-Routing konfigurieren")}
                </h2>
                <p className="studio-panel__description">{tr("Configure OpenAI GPT-5.6 or an OpenAI-compatible provider, then run a server-side DM test.", "Konfiguriere OpenAI GPT-5.6 oder einen OpenAI-kompatiblen Anbieter und starte anschließend einen serverseitigen Spielleiter-Test.")}</p>
              </div>
              <button className="icon-button" aria-label={tr("Close AI model settings", "KI-Modelleinstellungen schließen")} onClick={handleCloseLlmModal} type="button">
                <X size={18} />
              </button>
            </div>

            <div className="camera-device-panel">
              <div className="form-grid">
                <input
                  className="studio-input"
                  onChange={(event) => setLlmBaseUrl(event.target.value)}
                  placeholder="https://api.openai.com/v1"
                  value={llmBaseUrl}
                />
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void handleFetchModels()} type="button">
                    {tr("Fetch Models", "Modelle abrufen")}
                  </button>
                  <button className="studio-button studio-button--ghost" disabled={llmTestStatus === "running"} onClick={() => void handleLlmConnectionTest()} type="button">
                    {llmTestStatus === "running" ? tr("Testing LLM...", "LLM wird getestet …") : tr("Run DM Test", "Spielleiter-Test starten")}
                  </button>
                  <button className="studio-button studio-button--ghost" onClick={handlePlayerScreenTest} type="button">
                    {tr("Player Screen Test", "Spieleransicht testen")}
                  </button>
                </div>
                <select onChange={(event) => setLlmModelInput(event.target.value)} value={llmModelInput}>
                  <option value="">{availableModels.length === 0 ? tr("No fetched models yet", "Noch keine Modelle abgerufen") : tr("Choose a model", "Modell auswählen")}</option>
                  {llmModelInput && !availableModels.includes(llmModelInput) ? <option value={llmModelInput}>{llmModelInput}</option> : null}
                  {availableModels.map((model) => (
                    <option key={model} value={model}>
                      {model}
                    </option>
                  ))}
                </select>
                <div className="list-stack">
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Brain size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("Current routing", "Aktuelles Routing")}</strong>
                      <p>{savedLlmModel || tr("No saved model", "Kein Modell gespeichert")} @ {savedLlmBaseUrl || tr("No saved base URL", "Keine Basis-URL gespeichert")}</p>
                    </div>
                    <StatusPill tone={llmConfigured ? "ready" : "warning"}>{statusLabel(llmConfigured ? "saved" : "missing")}</StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <PlayCircle size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("DM test result", "Ergebnis des Spielleiter-Tests")}</strong>
                      <p>{llmTestMessage}</p>
                    </div>
                    <StatusPill tone={llmTestStatus === "success" ? "ready" : llmTestStatus === "error" ? "warning" : "info"}>
                      {statusLabel(llmTestStatus === "running" ? "running" : llmTestStatus)}
                    </StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Monitor size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>{tr("Player screen test result", "Testergebnis der Spieleransicht")}</strong>
                      <p>{playerScreenTestMessage}</p>
                    </div>
                    <StatusPill tone={playerScreenTestStatus === "success" ? "ready" : "info"}>{statusLabel(playerScreenTestStatus)}</StatusPill>
                  </article>
                </div>
              </div>
            </div>

            <div className="modal-card__footer">
              {llmSaveNotice ? <p className="camera-save-notice">{llmSaveNotice}</p> : <span className="modal-card__spacer" />}
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={handleCloseLlmModal} type="button">
                  {tr("Cancel", "Abbrechen")}
                </button>
                <button className="studio-button" onClick={handleSaveLlmSettings} type="button">
                  {tr("Save LLM Settings", "LLM-Einstellungen speichern")}
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
