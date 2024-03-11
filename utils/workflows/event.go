package wfutils

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

func PublishEvent[T any](ctx workflow.Context, eventName string, data T) error {
	event := cloudevents.NewEvent()
	event.SetID(uuid.NewString())
	event.SetSpecVersion(cloudevents.VersionV1)
	event.SetSource("bccm-flows")
	event.SetType(eventName)
	err := event.SetData(
		cloudevents.ApplicationJSON,
		data,
	)
	if err != nil {
		return err
	}

	return Execute[any, any](ctx, activities.PubsubPublish, event).Get(ctx, nil)
}
