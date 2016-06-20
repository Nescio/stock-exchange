package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
)

// redisWriter publishes messages to the Redis CHANNEL
type redisWriter struct {
	pool     *redis.Pool
	messages chan []byte
	channelName string
}

func newRedisWriter(pool *redis.Pool, channelName string) redisWriter {
	return redisWriter{
		pool:     pool,
		messages: make(chan []byte),
		channelName: channelName,
	}
}

// run the main redisWriter loop that publishes incoming messages to Redis.
func (rw *redisWriter) run() {
	conn := rw.pool.Get()
	defer conn.Close()

	for data := range rw.messages {
		ctx := log.Fields{"data": data}
		if err := conn.Send("PUBLISH", rw.channelName, data); err != nil {
			ctx["err"] = err
			log.WithFields(ctx).Fatalf("Unable to publish message to Redis")
		}
		if err := conn.Flush(); err != nil {
			ctx["err"] = err
			log.WithFields(ctx).Fatalf("Unable to flush published message to Redis")
		}
	}
}

// publish to Redis via channel.
func (rw *redisWriter) publish(data []byte) {
	rw.messages <- data
}
