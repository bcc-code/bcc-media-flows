package activities

import (
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
)

func PubsubPublish(ctx context.Context, data any) error {
	client, err := pubsub.NewClient(ctx, "btv-platform-prod-2")
	if err != nil {
		return err
	}

	topic := client.Topic("background_worker")
	defer topic.Stop()

	msg, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = topic.Publish(ctx, &pubsub.Message{
		Data: msg,
	}).Get(ctx)
	return err
}
