package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultChunkSeconds    = 8
	defaultPollSeconds     = 15
	defaultContextTTL      = 2 * time.Minute
	defaultContextMaxChars = 320
	defaultQuality         = "best"
	defaultStreamlinkCmd   = "streamlink"
	defaultFFMPEGCmd       = "ffmpeg"
)

// Config controls how stream audio is captured and transcribed.
type Config struct {
	Channel          string
	SourceURL        string
	Quality          string
	ChunkSeconds     int
	PollSeconds      int
	ContextTTL       time.Duration
	ContextMaxChars  int
	STTURL           string
	STTModel         string
	STTAPIKey        string
	STTLanguage      string
	StreamlinkBinary string
	FFMPEGBinary     string
	TwitchOAuthToken string
	OnContextUpdate  func(string)
}

type segment struct {
	Text string
	At   time.Time
}

// Service captures stream audio chunks, transcribes them and stores recent context.
type Service struct {
	cfg Config

	mu               sync.RWMutex
	segments         []segment
	lastTranscript   string
	cachedStreamURL  string
	streamURLCachedAt time.Time
}

func NewService(cfg Config) *Service {
	if cfg.ChunkSeconds <= 0 {
		cfg.ChunkSeconds = defaultChunkSeconds
	}
	if cfg.PollSeconds <= 0 {
		cfg.PollSeconds = defaultPollSeconds
	}
	if cfg.ContextTTL <= 0 {
		cfg.ContextTTL = defaultContextTTL
	}
	if cfg.ContextMaxChars <= 0 {
		cfg.ContextMaxChars = defaultContextMaxChars
	}
	if strings.TrimSpace(cfg.Quality) == "" {
		cfg.Quality = defaultQuality
	}
	if strings.TrimSpace(cfg.StreamlinkBinary) == "" {
		cfg.StreamlinkBinary = defaultStreamlinkCmd
	}
	if strings.TrimSpace(cfg.FFMPEGBinary) == "" {
		cfg.FFMPEGBinary = defaultFFMPEGCmd
	}
	return &Service{
		cfg:      cfg,
		segments: make([]segment, 0, 32),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.STTURL) == "" {
		return fmt.Errorf("audio stt url is empty")
	}
	if strings.TrimSpace(s.cfg.SourceURL) == "" && strings.TrimSpace(s.cfg.Channel) == "" {
		return fmt.Errorf("audio source is not configured: set channel or source url")
	}

	go s.loop(ctx)
	return nil
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.PollSeconds) * time.Second)
	defer ticker.Stop()

	s.captureAndTranscribe(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.captureAndTranscribe(ctx)
		}
	}
}

func (s *Service) captureAndTranscribe(ctx context.Context) {
	log.Printf("[audio] Starting capture and transcribe cycle")
	audioData, err := s.captureChunk(ctx)
	if err != nil {
		log.Printf("[audio] capture error: %v", err)
		return
	}
	if len(audioData) == 0 {
		log.Printf("[audio] captured audio is empty, skipping")
		return
	}
	log.Printf("[audio] captured %d bytes of audio", len(audioData))

	text, err := s.transcribe(ctx, audioData)
	if err != nil {
		log.Printf("[audio] stt error: %v", err)
		return
	}

	text = sanitizeTranscript(text)
	if text == "" {
		log.Printf("[audio] transcription result is empty after sanitization")
		return
	}

	log.Printf("[audio] transcription result: %s", text)

	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.EqualFold(text, s.lastTranscript) {
		log.Printf("[audio] duplicate transcript, skipping")
		return
	}

	now := time.Now()
	s.lastTranscript = text
	s.segments = append(s.segments, segment{Text: text, At: now})
	s.pruneLocked(now)
	log.Printf("[audio] added new segment, total segments: %d", len(s.segments))

	// Отправляем обновленный контекст через callback
	if s.cfg.OnContextUpdate != nil {
		context := s.getRecentContextLocked()
		s.cfg.OnContextUpdate(context)
	}
}

func (s *Service) GetRecentContext() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.pruneLocked(now)
	return s.getRecentContextLocked()
}

func (s *Service) getRecentContextLocked() string {
	if len(s.segments) == 0 {
		return ""
	}

	parts := make([]string, 0, len(s.segments))
	total := 0
	for i := len(s.segments) - 1; i >= 0; i-- {
		txt := s.segments[i].Text
		nextLen := total + len(txt)
		if len(parts) > 0 {
			nextLen += 3
		}
		if nextLen > s.cfg.ContextMaxChars {
			break
		}
		parts = append([]string{txt}, parts...)
		total = nextLen
	}
	return strings.Join(parts, " | ")
}

func (s *Service) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.cfg.ContextTTL)
	idx := 0
	for idx < len(s.segments) && s.segments[idx].At.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		s.segments = append([]segment(nil), s.segments[idx:]...)
	}
}

func (s *Service) captureChunk(ctx context.Context) ([]byte, error) {
	streamURL, err := s.resolveStreamURL(ctx)
	if err != nil {
		return nil, err
	}

	captureCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.ChunkSeconds+20)*time.Second)
	defer cancel()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", streamURL,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", strconv.Itoa(s.cfg.ChunkSeconds),
		"-f", "wav",
		"-",
	}
	cmd := exec.CommandContext(captureCtx, s.cfg.FFMPEGBinary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (s *Service) resolveStreamURL(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.cfg.SourceURL) != "" {
		return s.cfg.SourceURL, nil
	}

	s.mu.RLock()
	cachedURL := s.cachedStreamURL
	cachedAt := s.streamURLCachedAt
	s.mu.RUnlock()
	if cachedURL != "" && time.Since(cachedAt) < 5*time.Minute {
		return cachedURL, nil
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	target := "https://twitch.tv/" + strings.TrimPrefix(strings.TrimSpace(s.cfg.Channel), "#")

	args := []string{"--stream-url", target, s.cfg.Quality}

	// Добавляем OAuth токен если он есть
	if token := strings.TrimSpace(s.cfg.TwitchOAuthToken); token != "" {
		// Убираем префикс oauth: если он есть
		token = strings.TrimPrefix(token, "oauth:")
		args = append(args, "--twitch-api-header", "Authorization=Bearer "+token)
	}

	cmd := exec.CommandContext(resolveCtx, s.cfg.StreamlinkBinary, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("streamlink resolve failed: %w", err)
	}

	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("streamlink returned empty stream url")
	}

	s.mu.Lock()
	s.cachedStreamURL = url
	s.streamURLCachedAt = time.Now()
	s.mu.Unlock()

	return url, nil
}

func (s *Service) transcribe(ctx context.Context, audioData []byte) (string, error) {
	log.Printf("[audio] Starting transcription, audio size: %d bytes", len(audioData))
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("model", s.cfg.STTModel)
	if strings.TrimSpace(s.cfg.STTLanguage) != "" {
		_ = writer.WriteField("language", s.cfg.STTLanguage)
	}

	part, err := writer.CreateFormFile("file", "chunk.wav")
	if err != nil {
		return "", fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("write multipart audio: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	log.Printf("[audio] Sending request to STT service: %s", s.cfg.STTURL)
	req, err := http.NewRequestWithContext(reqCtx, "POST", s.cfg.STTURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(s.cfg.STTAPIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.cfg.STTAPIKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt http error: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[audio] STT response status: %d", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read stt response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("stt status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	log.Printf("[audio] STT response body: %s", string(respBody))

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err == nil && strings.TrimSpace(parsed.Text) != "" {
		return parsed.Text, nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return "", fmt.Errorf("unmarshal stt response: %w", err)
	}
	if t, ok := raw["text"].(string); ok {
		return t, nil
	}
	if t, ok := raw["result"].(string); ok {
		return t, nil
	}

	return "", nil
}

func sanitizeTranscript(in string) string {
	out := strings.TrimSpace(strings.ReplaceAll(in, "\n", " "))
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return out
}
