package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/team/websocket-poc/internal/odds"
)

type RedisPubSub struct {
	client *redis.Client
}

func NewRedisPubSub(addr string) *RedisPubSub {
	return &RedisPubSub{
		client: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

func (r *RedisPubSub) Publish(ctx context.Context, update odds.OddsUpdate) error {
	data, err := json.Marshal(update)
	if err != nil {
		return err
	}
	channel := fmt.Sprintf("odds:%s", update.MatchID)
	return r.client.Publish(ctx, channel, data).Err()
}

func (r *RedisPubSub) Subscribe(ctx context.Context, matchID string, handler func(odds.OddsUpdate)) error {
	channel := fmt.Sprintf("odds:%s", matchID)
	sub := r.client.Subscribe(ctx, channel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var update odds.OddsUpdate
			if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
				continue
			}
			handler(update)
		}
	}
}

func (r *RedisPubSub) SubscribePattern(ctx context.Context, pattern string, handler func(odds.OddsUpdate)) error {
	sub := r.client.PSubscribe(ctx, pattern)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			var update odds.OddsUpdate
			if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
				continue
			}
			handler(update)
		}
	}
}

func (r *RedisPubSub) Close() error {
	return r.client.Close()
}
