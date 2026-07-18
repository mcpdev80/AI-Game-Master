package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAITTSSendsSpeechRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization: %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "gpt-4o-mini-tts" || payload["voice"] != "cedar" || payload["response_format"] != "wav" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if payload["instructions"] != "Speak clearly." {
			t.Fatalf("OpenAI instructions missing: %#v", payload)
		}
		if _, exists := payload["instruct"]; exists {
			t.Fatal("local-provider instruct field leaked to OpenAI")
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("RIFF-test-audio"))
	}))
	defer server.Close()

	client := NewTTSClient(Config{
		TTSProvider: "openai", TTSBaseURL: server.URL + "/v1", TTSModel: "gpt-4o-mini-tts",
		TTSAPIKey: "test-key", TTSVoice: "cedar",
	})
	audio, contentType, err := client.Synthesize(context.Background(), "Open the gate.", client.VoiceForProfile("narrator-default"), "Speak clearly.")
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "RIFF-test-audio" || contentType != "audio/wav" {
		t.Fatalf("unexpected response: type=%q audio=%q", contentType, audio)
	}
}

func TestOpenAISTTSendsMultipartTranscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization: %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != "gpt-4o-transcribe" || r.FormValue("response_format") != "json" || r.FormValue("language") != "de" {
			t.Fatalf("unexpected fields: %#v", r.MultipartForm.Value)
		}
		if !strings.Contains(r.FormValue("prompt"), "fantasy terms") {
			t.Fatalf("missing domain prompt: %q", r.FormValue("prompt"))
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "test-wave" {
			t.Fatalf("unexpected audio: %q", data)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"Ich öffne das alte Tor."}`))
	}))
	defer server.Close()

	client := NewSTTClient(Config{
		STTProvider: "openai", STTBaseURL: server.URL + "/v1", STTModel: "gpt-4o-transcribe",
		STTAPIKey: "test-key", STTPrompt: "Preserve fantasy terms and dice notation.",
	})
	text, err := client.Transcribe(context.Background(), "speech.wav", "audio/wav", "de-DE", []byte("test-wave"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "Ich öffne das alte Tor." {
		t.Fatalf("unexpected transcript: %q", text)
	}
}

func TestOpenAIVoiceProfilesUseBuiltInVoices(t *testing.T) {
	client := NewTTSClient(Config{TTSProvider: "openai", TTSVoice: "cedar"})
	want := map[string]string{
		"narrator-default": "cedar",
		"npc-default":      "marin",
		"orc-deep":         "onyx",
		"elf-bright":       "shimmer",
		"rules-neutral":    "sage",
	}
	for profile, voice := range want {
		if got := client.VoiceForProfile(profile); got != voice {
			t.Fatalf("%s voice = %q, want %q", profile, got, voice)
		}
	}
}
