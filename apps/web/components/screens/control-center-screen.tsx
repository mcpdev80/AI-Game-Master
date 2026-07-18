"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { Brain, Camera, Database, Dices, Mic, Monitor, Network, PlayCircle, Radio, RefreshCw, Square, Volume2, Wifi, X } from "lucide-react";
import { PageIntro, Panel, StatCard, StatusPill } from "../studio-primitives";
import { detectDiceFromImage, fetchLLMModels, stabilizeDiceFrames, testLLMConnection, updateSystemConfig, type DiceBox, type DiceDetection, type DiceDetectionFrame, type LLMGatewayStatus } from "../../lib/api";
import type { PlayerLinkSlot, Session } from "../../lib/api";

type ControlCenterScreenProps = {
  services: { name: string; status: string }[];
  counts: Record<string, number>;
  llm: { base_url?: string; model?: string };
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

function generateGuidedRollPlan(): GuidedRollStep[] {
  const labels = ["attack roll", "damage roll", "saving throw", "spell effect", "check"];
  const types: DiceType[] = ["d4", "d6", "d8", "d10", "d12", "d20", "d100"];
  const stepCount = randomInt(1, 2);
  const steps: GuidedRollStep[] = [];

  for (let index = 0; index < stepCount; index += 1) {
    const type = types[randomInt(0, types.length - 1)];
    const count = type === "d20" ? randomInt(1, 2) : type === "d100" ? 1 : randomInt(1, 4);
    steps.push({
      id: crypto.randomUUID(),
      type,
      count,
      label: labels[randomInt(0, labels.length - 1)],
      status: index === 0 ? "active" : "pending",
      confirmedValues: [],
    });
  }

  return steps;
}

const checks = [
  { name: "Database", icon: Database, detail: "Postgres connected", tone: "ready" as const },
  { name: "Player Screen", icon: Monitor, detail: "Second display route ready", tone: "ready" as const },
  { name: "Player Portal", icon: Wifi, detail: "LAN access enabled", tone: "ready" as const },
  { name: "Network", icon: Network, detail: "Local network reachable", tone: "ready" as const },
];

export function ControlCenterScreen({ services, counts, llm, llmGateway, sessions, playerLinks }: ControlCenterScreenProps) {
  const onlineServices = services.filter((service) => service.status === "online").length;
  const liveSession = sessions.find((session) => session.status === "live") ?? sessions[0] ?? null;
  const joinedPlayers = playerLinks.filter((slot) => slot.player_slot.status === "joined").length;
  const readyTone =
    llm.model && counts.documents > 0 && counts.campaigns > 0 && liveSession ? "ready" : "warning";
  const [cameraDevices, setCameraDevices] = useState<MediaDeviceInfo[]>([]);
  const [selectedCameraId, setSelectedCameraId] = useState("");
  const [savedCameraId, setSavedCameraId] = useState("");
  const [cameraStatus, setCameraStatus] = useState<"idle" | "ready" | "error" | "unsupported">("idle");
  const [cameraMessage, setCameraMessage] = useState("No browser camera test has run yet.");
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
  const [diceTestMessage, setDiceTestMessage] = useState("No dice test has been run yet.");
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
  const [audioMessage, setAudioMessage] = useState("No microphone or speaker test has run yet.");
  const [audioSaveNotice, setAudioSaveNotice] = useState("");
  const [isTestingMicrophone, setIsTestingMicrophone] = useState(false);
  const [isAudioModalOpen, setIsAudioModalOpen] = useState(false);
  const [micLevel, setMicLevel] = useState(0);
  const [llmTestStatus, setLlmTestStatus] = useState<"idle" | "running" | "success" | "error">("idle");
  const [llmTestMessage, setLlmTestMessage] = useState("No model test has been run yet.");
  const [llmBaseUrl, setLlmBaseUrl] = useState(llm.base_url ?? "");
  const [savedLlmBaseUrl, setSavedLlmBaseUrl] = useState(llm.base_url ?? "");
  const [llmModelInput, setLlmModelInput] = useState(llm.model ?? "");
  const [savedLlmModel, setSavedLlmModel] = useState(llm.model ?? "");
  const [availableModels, setAvailableModels] = useState<string[]>([]);
  const [llmSaveNotice, setLlmSaveNotice] = useState("");
  const [isLlmModalOpen, setIsLlmModalOpen] = useState(false);
  const [playerScreenTestStatus, setPlayerScreenTestStatus] = useState<"idle" | "success">("idle");
  const [playerScreenTestMessage, setPlayerScreenTestMessage] = useState("No player screen test has been triggered yet.");
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

  const selectedCameraLabel = useMemo(
    () => cameraDevices.find((device) => device.deviceId === selectedCameraId)?.label || "Default browser camera",
    [cameraDevices, selectedCameraId]
  );
  const savedCameraLabel = useMemo(
    () => cameraDevices.find((device) => device.deviceId === savedCameraId)?.label || (cameraConfigured ? "Saved camera" : "Not configured"),
    [cameraConfigured, cameraDevices, savedCameraId]
  );
  const selectedMicrophoneLabel = useMemo(
    () => microphoneDevices.find((device) => device.deviceId === selectedMicrophoneId)?.label || "Default microphone",
    [microphoneDevices, selectedMicrophoneId]
  );
  const selectedSpeakerLabel = useMemo(
    () => speakerDevices.find((device) => device.deviceId === selectedSpeakerId)?.label || "Default browser output",
    [speakerDevices, selectedSpeakerId]
  );
  const savedMicrophoneLabel = useMemo(
    () =>
      microphoneDevices.find((device) => device.deviceId === savedMicrophoneId)?.label ||
      (savedMicrophoneId ? "Saved microphone" : "No microphone saved"),
    [microphoneDevices, savedMicrophoneId]
  );
  const savedSpeakerLabel = useMemo(
    () =>
      speakerDevices.find((device) => device.deviceId === savedSpeakerId)?.label ||
      (savedSpeakerId ? "Saved speaker" : "No speaker saved"),
    [savedSpeakerId, speakerDevices]
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
    setDiceTestMessage("No dice test has been run yet.");
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
    setAudioMessage("Microphone test stopped.");
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
      setCameraMessage("This browser does not expose camera device enumeration.");
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
      setCameraMessage("No camera devices were found in this browser.");
    }
  }

  async function refreshAudioDevices() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.enumerateDevices) {
      setAudioStatus("unsupported");
      setAudioMessage("This browser does not expose audio device enumeration.");
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
      setAudioMessage("No audio devices were found in this browser.");
    }
  }

  async function handleCameraTest() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setCameraStatus("unsupported");
      setCameraMessage("This browser does not support camera access.");
      return;
    }

    setIsTestingCamera(true);
    setCameraMessage("Requesting camera access...");
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
      setCameraMessage(`Live preview active from ${selectedCameraLabel}.`);
    } catch (error) {
      setCameraStatus("error");
      setCameraMessage(error instanceof Error ? error.message : "Camera access failed.");
    } finally {
      setIsTestingCamera(false);
    }
  }

  function handleStopCameraTest() {
    stopCameraStream();
    setCameraStatus("idle");
    setCameraMessage("Camera test stopped. The browser stream was released.");
  }

  async function handleMicrophoneTest() {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      setAudioStatus("unsupported");
      setAudioMessage("This browser does not support microphone access.");
      return;
    }

    setIsTestingMicrophone(true);
    setAudioMessage("Requesting microphone access...");
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
      setAudioMessage(`Microphone live from ${selectedMicrophoneLabel}.`);
    } catch (error) {
      setAudioStatus("error");
      setAudioMessage(error instanceof Error ? error.message : "Microphone access failed.");
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
      setAudioMessage(`Speaker test played on ${selectedSpeakerLabel}.`);
    } catch (error) {
      setAudioStatus("error");
      setAudioMessage(error instanceof Error ? error.message : "Speaker test failed.");
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
    const message = selectedCameraId ? `Saved camera preference: ${selectedCameraLabel}.` : "Camera preference cleared.";
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
      setDiceTestMessage("Start the camera test before capturing a dice frame.");
      return;
    }

    setDiceStabilityStatus("stabilizing");
    setIsCapturingDice(true);
    setDiceTestMessage("Capturing multiple frames and checking for a stable dice result...");
    try {
      if (showDiceDebugInput && diceFrameDraft.length > 0) {
        const nextFrame: DiceDetectionFrame = {
          frame_id: crypto.randomUUID(),
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
          setDiceTestMessage(`Debug dice recognized. Stabilizing: ${stabilized.matching_frames}/${stabilized.required_matches}.`);
          return;
        }

        setDetectedDice(stabilized.stable_dice);
        setDetectedDiceCount(stabilized.stable_dice.length);
        setEditableDetectedDice(stabilized.stable_dice.map((die) => die.value));
        setDiceStabilityStatus("stable");
        setDiceTestMessage(`Stable debug result: ${stabilized.stable_dice.map((die) => `${die.type} zeigt ${die.value}`).join(", ")}.`);
        return;
      }

      const capturedFrames: DiceDetectionFrame[] = [];
      const detectionNotes: string[] = [];
      const detectionImages: string[] = [];
      const detectionBoxes: DiceBox[][] = [];
      for (let index = 0; index < 3; index += 1) {
        const imageDataURL = captureCurrentFrame();
        if (!imageDataURL) {
          throw new Error("No live frame available yet. Wait for the preview and try again.");
        }

        const detection = await detectDiceFromImage({
          image_data_url: imageDataURL,
          language: "de",
        });
        detectionNotes.push(detection.notes);
        detectionImages.push(imageDataURL);
        detectionBoxes.push(detection.boxes);
        capturedFrames.push({
          frame_id: crypto.randomUUID(),
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
        setDiceTestMessage(fallbackNote || "No clear dice were recognized across the captured frames.");
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
          `Dice were seen, but not yet stable enough. Matching frames: ${stabilized.matching_frames}/${stabilized.required_matches}. Try keeping the dice still and capture again.`
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
            ? `Step erkannt: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} fuer ${activeGuidedRollStep.label}. Pruefe die Werte und bestaetige sie.`
            : `Teilweise erkannt: erwartet ${activeGuidedRollStep.count}${activeGuidedRollStep.type}, gelesen ${matched}. Bitte Werte pruefen oder korrigieren.`
        );
      } else {
        setDiceTestMessage(`Wurf erkannt: ${stableDice.map((die) => `${die.type} zeigt ${die.value}`).join(", ")}.`);
      }
    } catch (error) {
      setDiceStabilityStatus("error");
      setDiceTestMessage(error instanceof Error ? error.message : "Dice test failed.");
    } finally {
      setIsCapturingDice(false);
    }
  }

  function startGuidedDiceTest() {
    const plan = generateGuidedRollPlan();
    resetDiceTest();
    setIsDiceTestActive(true);
    setGuidedRollPlan(plan);
    const first = plan[0];
    setDiceTestMessage(`Step 1/${plan.length}: Bitte wuerfle jetzt ${first.count}${first.type} fuer ${first.label}.`);
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
      setDiceTestMessage(`Bitte trage fuer ${activeGuidedRollStep.count}${activeGuidedRollStep.type} alle Werte ein, bevor du bestaetigst.`);
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
      setDiceTestMessage(`Naechster Schritt: Bitte wuerfle jetzt ${nextActive.count}${nextActive.type} fuer ${nextActive.label}.`);
      return;
    }

    setDiceStabilityStatus("stable");
    setDiceTestMessage("Guided dice test complete. All requested roll steps were confirmed.");
  }

  function handleSaveAudioSelection() {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(microphonePreferenceStorageKey, selectedMicrophoneId);
      window.localStorage.setItem(speakerPreferenceStorageKey, selectedSpeakerId);
    }
    setSavedMicrophoneId(selectedMicrophoneId);
    setSavedSpeakerId(selectedSpeakerId);
    const message = `Saved audio settings: ${selectedMicrophoneLabel} / ${selectedSpeakerLabel}.`;
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
        const message = `LLM aktiv: ${saved.llm_model} @ ${saved.llm_base_url}`;
        setLlmSaveNotice(message);
        setLlmTestMessage(message);
        window.setTimeout(() => {
          handleCloseLlmModal();
          window.location.reload();
        }, 500);
      } catch (error) {
        setLlmSaveNotice(error instanceof Error ? error.message : "LLM-Einstellungen konnten nicht gespeichert werden.");
      }
    })();
  }

  async function handleFetchModels() {
    if (!llmBaseUrl.trim()) {
      setLlmTestStatus("error");
      setLlmTestMessage("Enter a base URL before fetching models.");
      return;
    }
    setLlmTestStatus("running");
    setLlmTestMessage("Fetching available models...");
    try {
      const data = await fetchLLMModels({ llm_base_url: llmBaseUrl.trim(), llm_model: llmModelInput.trim() });
      const models = data.models;
      setAvailableModels(models);
      if (!llmModelInput && models[0]) {
        setLlmModelInput(models[0]);
      }
      setLlmTestStatus("success");
      setLlmTestMessage(models.length > 0 ? `${models.length} models loaded.` : "Model endpoint responded, but no models were listed.");
    } catch (error) {
      setLlmTestStatus("error");
      setLlmTestMessage(error instanceof Error ? error.message : "Fetching models failed.");
    }
  }

  async function handleLlmConnectionTest() {
    if (!llmBaseUrl.trim() || !llmModelInput.trim()) {
      setLlmTestStatus("error");
      setLlmTestMessage("Enter a base URL and choose a model before running the DM test.");
      return;
    }

    setLlmTestStatus("running");
    setLlmTestMessage("Sending a short GM prompt through the secure API backend...");
    try {
      const data = await testLLMConnection({ llm_base_url: llmBaseUrl.trim(), llm_model: llmModelInput.trim() });
      const content = data.content.trim();
      setLlmTestStatus("success");
      setLlmTestMessage(content || `${data.model || llmModelInput} responded successfully.`);
    } catch (error) {
      setLlmTestStatus("error");
      setLlmTestMessage(error instanceof Error ? error.message : "LLM test failed.");
    }
  }

  function handlePlayerScreenTest() {
    if (typeof window !== "undefined") {
      window.open("/player-screen", "_blank", "noopener,noreferrer");
    }
    setPlayerScreenTestStatus("success");
    setPlayerScreenTestMessage("Player screen opened in a new tab. Verify the routed display or projector output.");
  }

  const cameraTone: "default" | "ready" | "warning" | "live" | "info" =
    cameraStatus === "ready" || cameraConfigured ? "ready" : cameraStatus === "unsupported" || cameraStatus === "error" ? "warning" : "info";
  const cameraDetail =
    cameraStatus === "unsupported"
      ? "Browser security context is blocking camera APIs"
      : cameraConfigured
        ? `Configured: ${savedCameraLabel}`
        : "Open settings to choose and test a camera";
  const audioTone: "default" | "ready" | "warning" | "live" | "info" =
    audioStatus === "ready" || audioConfigured ? "ready" : audioStatus === "unsupported" || audioStatus === "error" ? "warning" : "info";
  const audioDetail =
    audioStatus === "unsupported"
      ? "Browser security context is blocking audio APIs"
      : audioConfigured
        ? `Configured: ${savedMicrophoneLabel} / ${savedSpeakerLabel}`
        : "Open settings to choose and test microphone and speakers";
  const llmConfigured = savedLlmBaseUrl.trim().length > 0 && savedLlmModel.trim().length > 0;
  const llmTone: "default" | "ready" | "warning" | "live" | "info" =
    llmTestStatus === "success" || llmConfigured ? "ready" : llmTestStatus === "error" ? "warning" : "info";
  const llmDetail =
    llmConfigured
      ? `${savedLlmModel} @ ${savedLlmBaseUrl}`
      : "Open settings to configure local model routing";

  return (
    <div className="page-stack">
      <PageIntro
        eyebrow="Control Center"
        title="Session readiness before the AI takes over"
        description="This is the operator surface. Devices, model reachability, player output paths, and live readiness stay visible here before you start a session."
        actions={
          <div className="button-row">
            <Link className="studio-button studio-button--ghost" href="/player-screen">
              Player Screen
            </Link>
            <Link className="studio-button" href={liveSession ? `/sessions/${liveSession.id}` : "/sessions"}>
              Open Live Session
            </Link>
          </div>
        }
      />

      <section className="hero-grid">
        <Panel
          title="System Health"
          description="A single place to confirm whether the local model, database, player outputs, and device chain are ready."
          className="hero-panel"
          action={<StatusPill tone={readyTone}>{readyTone === "ready" ? "Ready" : "Needs Setup"}</StatusPill>}
        >
          <div className="status-grid">
            <button className="status-card status-card--interactive" onClick={handleOpenCameraModal} type="button">
              <div className="status-card__icon">
                <Camera size={18} />
              </div>
              <div>
                <div className="status-card__head">
                  <strong>Camera & Dice</strong>
                  <StatusPill tone={cameraTone}>{cameraConfigured ? "Ready" : "Setup"}</StatusPill>
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
                  <strong>Audio & Microphone</strong>
                  <StatusPill tone={audioTone}>{audioConfigured ? "Ready" : "Setup"}</StatusPill>
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
                  <strong>AI Model · Powered by {savedLlmModel || "GPT-5.6"}</strong>
                  <StatusPill tone={llmTone}>{llmConfigured ? "Ready" : "Setup"}</StatusPill>
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
                      <StatusPill tone={item.tone}>Ready</StatusPill>
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
        <Panel title="Live Summary" description="Current content inventory and system footprint.">
          <div className="stat-grid">
            <StatCard label="Online services" value={`${onlineServices}/${services.length}`} detail="API and database verified" />
            <StatCard label="Campaigns" value={counts.campaigns ?? 0} />
            <StatCard label="Sessions" value={counts.sessions ?? 0} />
            <StatCard label="Documents" value={counts.documents ?? 0} />
            <StatCard label="Assets" value={counts.assets ?? 0} />
            <StatCard label="Characters" value={counts.characters ?? 0} />
            <StatCard label="Chunks" value={counts.document_chunks ?? 0} />
          </div>
        </Panel>

        {llmGateway ? (
          <Panel title="LLM Gateway" description="Readonly runtime guard for concurrency, breaker, and archived session state.">
            <div className="stat-grid">
              <StatCard label="Status" value={llmGateway.status} />
              <StatCard label="In Flight" value={llmGateway.in_flight} />
              <StatCard label="Max Concurrent" value={llmGateway.max_concurrent_requests} />
              <StatCard label="Rejected" value={llmGateway.rejected_requests} />
              <StatCard label="Timeouts" value={llmGateway.timeout_count} />
              <StatCard label="Active Sessions" value={llmGateway.active_gateway_sessions} />
              <StatCard label="Archived Sessions" value={llmGateway.archived_gateway_sessions} />
            </div>
            <div className="meta-chip-row">
              <StatusPill tone={llmGateway.circuit_breaker_open ? "warning" : "ready"}>
                {llmGateway.circuit_breaker_open ? "Circuit Open" : "Circuit Closed"}
              </StatusPill>
              <StatusPill tone="default">Queue {llmGateway.queue_length}</StatusPill>
              <StatusPill tone="default">Failures {llmGateway.consecutive_failures}</StatusPill>
            </div>
            {llmGateway.last_error ? <p className="muted-copy">{llmGateway.last_error}</p> : null}
            <div className="builder-documents-grid">
              {llmGateway.profiles.map((profile) => (
                <article className="builder-document-card" key={profile.name}>
                  <strong>{profile.name}</strong>
                  <p>Input {profile.max_input_tokens} · Output {profile.max_output_tokens}</p>
                  <p>Timeout {profile.timeout_seconds}s · Window {profile.live_turn_window}</p>
                </article>
              ))}
            </div>
          </Panel>
        ) : null}

        <Panel title="Session Readiness" description="Requirements for a live AI-led session.">
          <div className="list-stack">
            <article className="list-row">
              <div className="list-row__icon">
                <PlayCircle size={18} />
              </div>
              <div className="list-row__body">
                <strong>{liveSession ? liveSession.current_scene || "Session selected" : "No session selected"}</strong>
                <p>{liveSession ? `${liveSession.current_location || "No location"} · ${liveSession.status}` : "Create or start a session first."}</p>
              </div>
              <StatusPill tone={liveSession ? "live" : "warning"}>{liveSession ? liveSession.status : "missing"}</StatusPill>
            </article>
            <article className="list-row">
              <div className="list-row__icon">
                <Wifi size={18} />
              </div>
              <div className="list-row__body">
                <strong>Player portal join state</strong>
                <p>{joinedPlayers} joined players · {playerLinks.length} total invite slots</p>
              </div>
              <StatusPill tone={playerLinks.length > 0 ? "ready" : "warning"}>{playerLinks.length > 0 ? "links ready" : "no links"}</StatusPill>
            </article>
            <article className="list-row">
              <div className="list-row__icon">
                <Radio size={18} />
              </div>
              <div className="list-row__body">
                <strong>Content and model context</strong>
                <p>{counts.documents ?? 0} documents, {counts.assets ?? 0} assets, local model {llm.model ? "configured" : "missing"}</p>
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
                <p className="eyebrow">Camera Settings</p>
                <h2 className="studio-panel__title" id="camera-settings-title">
                  Configure camera and dice capture input
                </h2>
                <p className="studio-panel__description">Choose a preferred camera, test the live preview, then save that selection for future sessions.</p>
              </div>
              <button className="icon-button" aria-label="Close camera settings" onClick={handleCloseCameraModal} type="button">
                <X size={18} />
              </button>
            </div>

            <div className="camera-device-panel">
              <div className="form-grid">
                <select onChange={(event) => setSelectedCameraId(event.target.value)} value={selectedCameraId}>
                  <option value="">{cameraDevices.length === 0 ? "No camera devices detected" : "Default browser camera"}</option>
                  {cameraDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.label || `Camera ${device.deviceId.slice(0, 6)}`}
                    </option>
                  ))}
                </select>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void refreshCameraDevices()} type="button">
                    Detect Cameras
                  </button>
                  <button className="studio-button" disabled={isTestingCamera} onClick={() => void handleCameraTest()} type="button">
                    {isTestingCamera ? "Testing..." : "Start Camera Test"}
                  </button>
                  <button
                    className="studio-button studio-button--ghost"
                    disabled={cameraStatus !== "ready"}
                    onClick={handleStopCameraTest}
                    type="button"
                  >
                    <Square size={16} />
                    Stop Test
                  </button>
                </div>
                <div className="list-stack">
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Camera size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>Camera status</strong>
                      <p>{cameraMessage}</p>
                    </div>
                    <StatusPill tone={cameraStatus === "ready" ? "ready" : cameraStatus === "unsupported" ? "warning" : "info"}>
                      {cameraStatus}
                    </StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Radio size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>Saved preference</strong>
                      <p>{cameraConfigured ? savedCameraLabel : "No camera saved yet."}</p>
                    </div>
                    <StatusPill tone={cameraConfigured ? "ready" : "warning"}>{cameraConfigured ? "saved" : "missing"}</StatusPill>
                  </article>
                </div>
                <div className="hint-box">
                  <strong>Hint:</strong> Camera APIs usually work best on `https` or `http://localhost`. Use `Stop Test` to release the device after checking it.
                </div>
                <div className="hint-box">
                  <strong>Dice Test:</strong> The current MVP is tuned for physical <code>d6</code> dice. Start the camera, roll the dice into the frame, then capture. Mixed number-d6 and pip-d6 should work better now than the broader all-dice experiment.
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
                            Step {index + 1}: {step.count}
                            {step.type}
                          </strong>
                          <p>
                            {step.label}
                            {step.confirmedValues.length > 0 ? ` • confirmed: ${step.confirmedValues.join(", ")}` : ""}
                          </p>
                        </div>
                        <StatusPill tone={step.status === "confirmed" ? "ready" : step.status === "active" ? "info" : "warning"}>
                          {step.status}
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
                    <StatusPill tone="ready">Live Preview</StatusPill>
                  </div>
                ) : (
                  <div className="camera-placeholder">
                    <Camera size={28} />
                    <span>{cameraStatus === "unsupported" ? "Camera unsupported" : "No live preview yet"}</span>
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
                  <strong>Dice test</strong>
                  <p>{diceTestMessage}</p>
                </div>
                <StatusPill tone={diceStabilityStatus === "stable" ? "ready" : diceStabilityStatus === "error" ? "warning" : "info"}>
                  {diceStabilityStatus}
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
                  <img alt="Dice analysis snapshot" src={diceAnalysisImage} />
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
                  <StatusPill tone="info">{detectedDiceBoxes.length} boxes</StatusPill>
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
                Start Guided Dice Test
              </button>
              <button className="studio-button studio-button--ghost" disabled={!isDiceTestActive} onClick={resetDiceTest} type="button">
                <RefreshCw size={16} />
                Reset Dice Test
              </button>
              <button className="studio-button" disabled={!isDiceTestActive || cameraStatus !== "ready" || !activeGuidedRollStep} onClick={() => void handleCaptureDiceFrame()} type="button">
                Capture Dice Frame
              </button>
              <button
                className="studio-button studio-button--ghost"
                disabled={!activeGuidedRollStep || editableDetectedDice.length === 0}
                onClick={confirmGuidedRollStep}
                type="button"
              >
                Confirm Step
              </button>
            </div>

            <div className="list-stack">
              <article className="list-row">
                <div className="list-row__icon">
                  <Dices size={18} />
                </div>
                <div className="list-row__body">
                  <strong>Detected result</strong>
                  <p>
                    {activeGuidedRollStep
                      ? `Expected: ${activeGuidedRollStep.count}${activeGuidedRollStep.type} for ${activeGuidedRollStep.label}.`
                      : detectedDice.length > 0
                      ? `${detectedDiceCount || detectedDice.length} dice found. Read values: ${detectedDice.map((die) => `${die.type} shows ${die.value}`).join(", ")}`
                      : "No stable dice result yet."}
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
                {showDiceDebugInput ? "Hide Debug Input" : "Open Debug Input"}
              </button>
            </div>

            {showDiceDebugInput ? (
              <div className="page-stack">
                <div className="hint-box">
                  <strong>Debug Input:</strong> This is only for backend stabilizer testing until automatic optical dice recognition is connected to the live camera frames.
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
                        Remove
                      </button>
                    </article>
                  ))}
                </div>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={addDiceDraftRow} type="button">
                    Add Die
                  </button>
                </div>
              </div>
            ) : null}

            <div className="modal-card__footer">
              {cameraSaveNotice ? <p className="camera-save-notice">{cameraSaveNotice}</p> : <span className="modal-card__spacer" />}
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={handleCloseCameraModal} type="button">
                  Cancel
                </button>
                <button className="studio-button" onClick={handleSaveCameraSelection} type="button">
                  Save Camera Setting
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
                <p className="eyebrow">Audio Settings</p>
                <h2 className="studio-panel__title" id="audio-settings-title">
                  Configure sound and microphone input
                </h2>
                <p className="studio-panel__description">Choose a preferred microphone and browser output, test both, then save that selection for future sessions.</p>
              </div>
              <button className="icon-button" aria-label="Close audio settings" onClick={handleCloseAudioModal} type="button">
                <X size={18} />
              </button>
            </div>

            <div className="camera-device-panel">
              <div className="form-grid">
                <select onChange={(event) => setSelectedMicrophoneId(event.target.value)} value={selectedMicrophoneId}>
                  <option value="">{microphoneDevices.length === 0 ? "No microphones detected" : "Default microphone"}</option>
                  {microphoneDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.label || `Microphone ${device.deviceId.slice(0, 6)}`}
                    </option>
                  ))}
                </select>
                <select onChange={(event) => setSelectedSpeakerId(event.target.value)} value={selectedSpeakerId}>
                  <option value="">{speakerDevices.length === 0 ? "No speakers detected" : "Default browser output"}</option>
                  {speakerDevices.map((device) => (
                    <option key={device.deviceId} value={device.deviceId}>
                      {device.label || `Speaker ${device.deviceId.slice(0, 6)}`}
                    </option>
                  ))}
                </select>
                <div className="button-row">
                  <button className="studio-button studio-button--ghost" onClick={() => void refreshAudioDevices()} type="button">
                    Detect Audio Devices
                  </button>
                  <button className="studio-button" disabled={isTestingMicrophone} onClick={() => void handleMicrophoneTest()} type="button">
                    {isTestingMicrophone ? "Testing..." : "Start Microphone Test"}
                  </button>
                  <button className="studio-button studio-button--ghost" onClick={() => void handleSpeakerTest()} type="button">
                    Play Test Sound
                  </button>
                  <button className="studio-button studio-button--ghost" disabled={audioStatus !== "ready"} onClick={stopMicrophoneTest} type="button">
                    <Square size={16} />
                    Stop Test
                  </button>
                </div>
                <div className="list-stack">
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Mic size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>Audio status</strong>
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
                      <strong>Saved preference</strong>
                      <p>{savedMicrophoneLabel} / {savedSpeakerLabel}</p>
                    </div>
                    <StatusPill tone={audioConfigured ? "ready" : "warning"}>{audioConfigured ? "saved" : "missing"}</StatusPill>
                  </article>
                </div>
                <div className="hint-box">
                  <strong>Hint:</strong> Browser speaker routing support depends on `setSinkId`. If not supported, the test sound will use the browser default output.
                </div>
              </div>
              <div className="audio-meter-card">
                <div className="audio-meter-card__head">
                  <strong>Microphone level</strong>
                  <StatusPill tone={audioStatus === "ready" ? "ready" : "info"}>{Math.round(micLevel)}%</StatusPill>
                </div>
                <div className="audio-meter">
                  <div className="audio-meter__fill" style={{ width: `${micLevel}%` }} />
                </div>
                <p className="studio-panel__description">Use this as a quick browser-level signal check before dice or voice capture.</p>
              </div>
            </div>

            <div className="modal-card__footer">
              {audioSaveNotice ? <p className="camera-save-notice">{audioSaveNotice}</p> : <span className="modal-card__spacer" />}
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={handleCloseAudioModal} type="button">
                  Cancel
                </button>
                <button className="studio-button" onClick={handleSaveAudioSelection} type="button">
                  Save Audio Settings
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
                <p className="eyebrow">AI Model Settings</p>
                <h2 className="studio-panel__title" id="llm-settings-title">
                  Configure AI model routing
                </h2>
                <p className="studio-panel__description">Configure OpenAI GPT-5.6 or an optional OpenAI-compatible local provider, then run a server-side GM response test.</p>
              </div>
              <button className="icon-button" aria-label="Close AI model settings" onClick={handleCloseLlmModal} type="button">
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
                    Fetch Models
                  </button>
                  <button className="studio-button studio-button--ghost" disabled={llmTestStatus === "running"} onClick={() => void handleLlmConnectionTest()} type="button">
                    {llmTestStatus === "running" ? "Testing LLM..." : "Run DM Test"}
                  </button>
                  <button className="studio-button studio-button--ghost" onClick={handlePlayerScreenTest} type="button">
                    Player Screen Test
                  </button>
                </div>
                <select onChange={(event) => setLlmModelInput(event.target.value)} value={llmModelInput}>
                  <option value="">{availableModels.length === 0 ? "No fetched models yet" : "Choose a model"}</option>
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
                      <strong>Current routing</strong>
                      <p>{savedLlmModel || "No saved model"} @ {savedLlmBaseUrl || "No saved base URL"}</p>
                    </div>
                    <StatusPill tone={llmConfigured ? "ready" : "warning"}>{llmConfigured ? "saved" : "missing"}</StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <PlayCircle size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>DM test result</strong>
                      <p>{llmTestMessage}</p>
                    </div>
                    <StatusPill tone={llmTestStatus === "success" ? "ready" : llmTestStatus === "error" ? "warning" : "info"}>
                      {llmTestStatus === "running" ? "running" : llmTestStatus}
                    </StatusPill>
                  </article>
                  <article className="list-row">
                    <div className="list-row__icon">
                      <Monitor size={18} />
                    </div>
                    <div className="list-row__body">
                      <strong>Player screen test result</strong>
                      <p>{playerScreenTestMessage}</p>
                    </div>
                    <StatusPill tone={playerScreenTestStatus === "success" ? "ready" : "info"}>{playerScreenTestStatus}</StatusPill>
                  </article>
                </div>
              </div>
            </div>

            <div className="modal-card__footer">
              {llmSaveNotice ? <p className="camera-save-notice">{llmSaveNotice}</p> : <span className="modal-card__spacer" />}
              <div className="button-row modal-card__actions">
                <button className="studio-button studio-button--ghost" onClick={handleCloseLlmModal} type="button">
                  Cancel
                </button>
                <button className="studio-button" onClick={handleSaveLlmSettings} type="button">
                  Save LLM Settings
                </button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}
