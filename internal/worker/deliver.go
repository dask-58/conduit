package worker

import (
	"fmt"

	"github.com/dask-58/conduit/internal/destination"
	"github.com/dask-58/conduit/internal/model"
)

func Deliver(job model.DeliveryJob, endpoint model.Endpoint) (int, error) {
	deliveryDestination := destination.Resolve(endpoint.URL)
	if err := deliveryDestination.Send(job.Payload); err != nil {
		return destination.LastStatus(deliveryDestination), fmt.Errorf("send delivery: %w", err)
	}

	return destination.LastStatus(deliveryDestination), nil
}
