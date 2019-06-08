package main

import (
	"context"

	"cloud.google.com/go/pubsub"
)

// queue pushes events to pubsub topic
type queue struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// configQueue is invoked once per Storable life cycle to configure the store
func configQueue(ctx context.Context, projectID, topicName string) {

	if projectID == "" {
		logger.Fatal("projectID not set")
	}

	if topicName == "" {
		logger.Fatal("topicName not set")
	}

	if ctx == nil {
		logger.Fatal("context not set")
	}

	c, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		logger.Fatalf("error while crating pubsub client: %v", err)
	}

	t := c.Topic(topicName)
	topicExists, err := t.Exists(ctx)
	if err != nil {
		logger.Fatalf("pubsub topic does not exists: %v", err)
	}

	if !topicExists {
		logger.Printf("Topic %s not found, creating...", topicName)
		t, err = c.CreateTopic(ctx, topicName)
		if err != nil {
			logger.Fatalf("Unable to create topic: %s - %v", topicName, err)
		}
	}

	que = &queue{
		client: c,
		topic:  t,
	}

}

// push persist the content
func (q *queue) push(ctx context.Context, data []byte) error {
	msg := &pubsub.Message{Data: data}
	result := q.topic.Publish(ctx, msg)
	_, err := result.Get(ctx)
	return err
}
