package agnes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithExtraFields(t *testing.T) {
	t.Run("forwards extra fields into request body", func(t *testing.T) {
		var capturedBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &capturedBody))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":"chatcmpl-test",
				"object":"chat.completion",
				"created":0,
				"model":"agnes-2.0-flash",
				"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
			}`))
		}))
		defer server.Close()

		cm, err := NewChatModel(context.Background(), &ChatModelConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
			Model:   "agnes-2.0-flash",
		})
		require.NoError(t, err)

		extra := map[string]any{
			"chat_template_kwargs": map[string]any{
				"thinking": true,
			},
		}

		_, err = cm.Generate(context.Background(), []*schema.Message{
			schema.UserMessage("hello"),
		}, WithExtraFields(extra))
		require.NoError(t, err)
		require.NotNil(t, capturedBody)

		got, ok := capturedBody["chat_template_kwargs"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, got["thinking"])
	})

	t.Run("nil extra fields is a no-op", func(t *testing.T) {
		var capturedBody map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &capturedBody))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":"chatcmpl-test",
				"object":"chat.completion",
				"created":0,
				"model":"agnes-2.0-flash",
				"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
			}`))
		}))
		defer server.Close()

		cm, err := NewChatModel(context.Background(), &ChatModelConfig{
			APIKey:  "test-key",
			BaseURL: server.URL + "/v1",
			Model:   "agnes-2.0-flash",
		})
		require.NoError(t, err)

		_, err = cm.Generate(context.Background(), []*schema.Message{
			schema.UserMessage("hello"),
		}, WithExtraFields(nil))
		require.NoError(t, err)
		require.NotNil(t, capturedBody)
		_, ok := capturedBody["chat_template_kwargs"]
		assert.False(t, ok)
	})
}
