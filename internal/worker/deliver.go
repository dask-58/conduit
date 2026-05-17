package worker

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dask-58/conduit/internal/model"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func Deliver(job model.DeliveryJob, endpoint model.Endpoint) (int, error) {
	resp, err := httpClient.Post(endpoint.URL, "application/json", bytes.NewReader(job.Payload))
	if err != nil {
		return 0, fmt.Errorf("post delivery: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return resp.StatusCode, fmt.Errorf("discard delivery response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, fmt.Errorf("delivery returned status %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}
