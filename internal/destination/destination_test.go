package destination

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDiscordDestination(t *testing.T) {
	destination := Resolve("https://discord.com/api/webhooks/123/abc")

	_, ok := destination.(*DiscordDestination)
	require.Truef(t, ok, "Resolve() returned %T, want *DiscordDestination", destination)
}

func TestResolveLegacyDiscordDestination(t *testing.T) {
	destination := Resolve("https://discordapp.com/api/webhooks/123/abc")

	_, ok := destination.(*DiscordDestination)
	require.Truef(t, ok, "Resolve() returned %T, want *DiscordDestination", destination)
}

func TestResolveGenericDestination(t *testing.T) {
	destination := Resolve("https://example.com/webhook")

	_, ok := destination.(*GenericDestination)
	require.Truef(t, ok, "Resolve() returned %T, want *GenericDestination", destination)
}

func TestDiscordDestinationSendPushPayload(t *testing.T) {
	var received discordMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	payload := []byte(`{
		"ref": "refs/heads/master",
		"repository": {
			"name": "mcpbox",
			"html_url": "https://github.com/dask-58/mcpbox"
		},
		"pusher": {
			"name": "dask-58"
		},
		"head_commit": {
			"message": "ship day 10"
		}
	}`)

	destination := &DiscordDestination{
		url:        server.URL,
		httpClient: server.Client(),
	}

	err := destination.Send(payload)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, destination.LastStatus())

	require.Len(t, received.Embeds, 1)

	embed := received.Embeds[0]
	assert.Equal(t, "push · mcpbox · master", embed.Title)
	assert.Equal(t, "ship day 10", embed.Description)
	assert.Equal(t, "https://github.com/dask-58/mcpbox", embed.URL)
	assert.Equal(t, discordEmbedColor, embed.Color)
	assert.Equal(t, "pushed by dask-58", embed.Footer.Text)
}

func TestBuildDiscordMessageFromGitHubPushPayload(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/master",
		"pusher": {
			"name": "dask-58",
			"email": "140686560+dask-58@users.noreply.github.com"
		},
		"repository": {
			"name": "mcpbox",
			"html_url": "https://github.com/dask-58/mcpbox"
		},
		"head_commit": {
			"message": "test: conduit smoke test 5"
		}
	}`)

	message, err := buildDiscordMessage(payload)
	require.NoError(t, err)
	require.Len(t, message.Embeds, 1)

	embed := message.Embeds[0]
	assert.Equal(t, "push · mcpbox · master", embed.Title)
	assert.Equal(t, "test: conduit smoke test 5", embed.Description)
	assert.Equal(t, "https://github.com/dask-58/mcpbox", embed.URL)
	assert.Equal(t, discordEmbedColor, embed.Color)
	assert.Equal(t, "pushed by dask-58", embed.Footer.Text)
}
