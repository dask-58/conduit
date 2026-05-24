package destination

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Destination interface {
	Send(payload []byte) error
}

type statusRecorder interface {
	LastStatus() int
}

type GenericDestination struct {
	url        string
	httpClient *http.Client
	lastStatus int
}

func Resolve(url string) Destination {
	if strings.Contains(strings.ToLower(url), "discord.com") {
		return &DiscordDestination{
			url:        url,
			httpClient: defaultHTTPClient,
		}
	}

	return &GenericDestination{
		url:        url,
		httpClient: defaultHTTPClient,
	}
}

func LastStatus(destination Destination) int {
	recorder, ok := destination.(statusRecorder)
	if !ok {
		return 0
	}

	return recorder.LastStatus()
}

func (d *GenericDestination) Send(payload []byte) error {
	resp, err := d.httpClient.Post(d.url, "application/json", bytes.NewReader(payload))
	if err != nil {
		d.lastStatus = 0
		return fmt.Errorf("post delivery: %w", err)
	}
	defer resp.Body.Close()

	d.lastStatus = resp.StatusCode

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("discard delivery response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("delivery returned status %d", resp.StatusCode)
	}

	return nil
}

func (d *GenericDestination) LastStatus() int {
	return d.lastStatus
}

var defaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}
