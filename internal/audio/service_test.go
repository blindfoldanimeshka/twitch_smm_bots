package audio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranscribeSendsWhisperFields(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20))
		got = map[string]string{
			"model":           r.FormValue("model"),
			"language":        r.FormValue("language"),
			"response_format": r.FormValue("response_format"),
		}
		_, file, err := r.FormFile("file")
		require.NoError(t, err)
		require.NotNil(t, file)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"привет"}`))
	}))
	defer srv.Close()

	s := NewService(Config{STTURL: srv.URL, STTModel: "whisper-1", STTLanguage: "ru"})
	text, err := s.transcribe(context.Background(), []byte("wav"))
	require.NoError(t, err)
	assert.Equal(t, "привет", text)
	assert.Equal(t, "whisper-1", got["model"])
	assert.Equal(t, "ru", got["language"])
	assert.Equal(t, "json", got["response_format"])
}

func TestTranscribeEmptyTextIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":""}`))
	}))
	defer srv.Close()

	s := NewService(Config{STTURL: srv.URL})
	text, err := s.transcribe(context.Background(), []byte("wav"))
	require.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestTranscribeNonJSONIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := NewService(Config{STTURL: srv.URL})
	_, err := s.transcribe(context.Background(), []byte("wav"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal stt response")
}
