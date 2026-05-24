package destination

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const discordEmbedColor = 3066993

const (
	discordContentLimit          = 2000
	discordEmbedTotalCharLimit   = 6000
	discordEmbedTitleLimit       = 256
	discordEmbedDescriptionLimit = 2048
	discordEmbedFooterLimit      = 2048
)

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
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	URL         string         `json:"url,omitempty"`
	Color       int            `json:"color"`
	Footer      *discordFooter `json:"footer,omitempty"`
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
	var footerText string
	content := "GitHub activity"

	switch detectEventType(event) {
	case "push":
		data := map[string]interface{}(event)
		ref, _ := data["ref"].(string)
		branch := strings.TrimPrefix(ref, "refs/heads/")

		var commitMsg, pusherName, repoName, repoURL string

		if hc, ok := data["head_commit"].(map[string]interface{}); ok {
			commitMsg, _ = hc["message"].(string)
		}
		if p, ok := data["pusher"].(map[string]interface{}); ok {
			pusherName, _ = p["name"].(string)
		}
		if r, ok := data["repository"].(map[string]interface{}); ok {
			repoName, _ = r["name"].(string)
			repoURL, _ = r["html_url"].(string)
		}

		title = fmt.Sprintf("push · %s · %s", fallback(repoName, "unknown"), fallback(branch, "unknown"))
		description = fallback(commitMsg, "unknown")
		url = repoURL
		footerText = fmt.Sprintf("pushed by %s", fallback(pusherName, "unknown"))
		content = fmt.Sprintf("%s\n%s", title, description)
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
		content = fmt.Sprintf("%s\n%s", title, description)
	case "ping":
		title = fmt.Sprintf("✅ webhook connected · %s", fallback(repoName, "unknown"))
		description = fallback(stringValue(event["zen"]), "unknown")
		color = 3447003
		content = fmt.Sprintf("%s\n%s", title, description)
	}

	title = truncateString(title, discordEmbedTitleLimit)
	description = truncateString(description, discordEmbedDescriptionLimit)
	footerText = truncateString(footerText, discordEmbedFooterLimit)
	content = truncateString(content, discordContentLimit)
	url = sanitizeDiscordURL(url)

	embed := discordEmbed{
		Title:       title,
		Description: description,
		URL:         url,
		Color:       color,
	}
	if footerText != "" {
		embed.Footer = &discordFooter{Text: footerText}
	}
	embed = trimEmbedToTotalLimit(embed)

	return discordMessage{
		Content: content,
		Embeds:  []discordEmbed{embed},
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

func sanitizeDiscordURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ""
	}

	if parsedURL.Host == "" {
		return ""
	}

	return rawURL
}

func truncateString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return value[:limit]
}

func trimEmbedToTotalLimit(embed discordEmbed) discordEmbed {
	total := len(embed.Title) + len(embed.Description)
	if embed.Footer != nil {
		total += len(embed.Footer.Text)
	}

	if total <= discordEmbedTotalCharLimit {
		return embed
	}

	overflow := total - discordEmbedTotalCharLimit
	if overflow >= len(embed.Description) {
		embed.Description = ""
		return embed
	}

	embed.Description = embed.Description[:len(embed.Description)-overflow]
	return embed
}
