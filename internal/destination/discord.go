package destination

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const discordEmbedColor = 3066993

type DiscordDestination struct {
	url        string
	httpClient *http.Client
	lastStatus int
}

func (d *DiscordDestination) Send(payload []byte) error {
	messagePayload, err := buildDiscordMessage(payload)
	if err != nil {
		d.lastStatus = 0
		return err
	}

	body, err := json.Marshal(messagePayload)
	if err != nil {
		d.lastStatus = 0
		return fmt.Errorf("marshal discord message: %w", err)
	}

	resp, err := d.httpClient.Post(d.url, "application/json", bytes.NewReader(body))
	if err != nil {
		d.lastStatus = 0
		return fmt.Errorf("post discord delivery: %w", err)
	}
	defer resp.Body.Close()

	d.lastStatus = resp.StatusCode

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("discard discord response: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord delivery returned status %d", resp.StatusCode)
	}

	return nil
}

func (d *DiscordDestination) LastStatus() int {
	return d.lastStatus
}

type discordMessage struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	URL         string        `json:"url,omitempty"`
	Color       int           `json:"color"`
	Footer      discordFooter `json:"footer"`
}

type discordFooter struct {
	Text string `json:"text"`
}

func buildDiscordMessage(payload []byte) (discordMessage, error) {
	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		return discordMessage{}, fmt.Errorf("unmarshal discord payload: %w", err)
	}

	repoName := nestedString(event, "repository", "name")
	title := "github event"
	description := "New GitHub activity."
	color := discordEmbedColor
	url := ""
	footer := discordFooter{Text: "unknown"}

	switch detectEventType(event) {
	case "push":
		repository := nestedMap(event, "repository")
		headCommit := nestedMap(event, "head_commit")
		pusher := nestedMap(event, "pusher")

		repoName = fallback(stringValue(repository["name"]), "unknown")
		branch := fallback(branchName(event), "unknown")
		description = fallback(stringValue(headCommit["message"]), "unknown")
		url = fallback(stringValue(repository["html_url"]), "unknown")
		pusherName := fallback(stringValue(pusher["name"]), "unknown")

		title = fmt.Sprintf("push · %s · %s", repoName, branch)
		footer = discordFooter{Text: fmt.Sprintf("pushed by %s", pusherName)}
	case "pull_request":
		action := stringValue(event["action"])
		if action == "" {
			action = "updated"
		}
		title = fmt.Sprintf("pull request %s · %s", action, fallback(repoName, "repository"))
		description = nestedString(event, "pull_request", "title")
		if description == "" {
			description = "Pull request activity."
		}
	case "ping":
		title = fmt.Sprintf("✅ webhook connected · %s", fallback(repoName, "unknown"))
		description = fallback(stringValue(event["zen"]), "unknown")
		color = 3447003
		footer = discordFooter{Text: "unknown"}
	}

	return discordMessage{
		Embeds: []discordEmbed{
			{
				Title:       title,
				Description: description,
				URL:         url,
				Color:       color,
				Footer:      footer,
			},
		},
	}, nil
}

func detectEventType(event map[string]any) string {
	if _, ok := event["ref"]; ok {
		return "push"
	}
	if _, ok := event["pull_request"]; ok {
		return "pull_request"
	}
	if _, ok := event["zen"]; ok {
		return "ping"
	}
	if action := stringValue(event["action"]); action != "" {
		return action
	}
	return "generic"
}

func branchName(event map[string]any) string {
	ref := stringValue(event["ref"])
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref
}

func nestedString(data map[string]any, path ...string) string {
	if len(path) == 0 {
		return ""
	}
	current := nestedMap(data, path[:len(path)-1]...)

	return stringValue(current[path[len(path)-1]])
}

func nestedMap(data map[string]any, path ...string) map[string]any {
	current := data
	for _, key := range path {
		value, ok := current[key]
		if !ok {
			return map[string]any{}
		}

		next, ok := value.(map[string]any)
		if !ok {
			return map[string]any{}
		}

		current = next
	}

	return current
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func fallback(value string, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
