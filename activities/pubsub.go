package activities

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub"
)

func (ua UtilActivities) PubsubPublish(ctx context.Context, data any) (any, error) {
	client, err := pubsub.NewClient(ctx, "btv-platform-prod-2")
	if err != nil {
		return nil, err
	}

	topic := client.Topic("background_worker")
	defer topic.Stop()

	msg, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	_, err = topic.Publish(ctx, &pubsub.Message{
		Data: msg,
	}).Get(ctx)
	return nil, err
}
