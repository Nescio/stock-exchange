package main

import (
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
)

// redisReceiver receives messages from Redis and broadcasts them to all
// registered websocket connections that are Registered.
type redisReceiver struct {
	pool        *redis.Pool
	sync.Mutex  // Protects the conns map
	conns       map[string]*websocket.Conn
	channelName string
}

// newRedisReceiver creates a redisReceiver that will use the provided
// rredis.Pool.
func newRedisReceiver(pool *redis.Pool, channelName string) *redisReceiver {
	return &redisReceiver{
		pool:  pool,
		conns: make(map[string]*websocket.Conn),
		channelName: channelName,
	}
}

// run receives pubsub messages from Redis after establishing a connection.
// When a valid message is received it is broadcast to all connected websockets
func (rr *redisReceiver) run() {
	conn := rr.pool.Get()
	defer conn.Close()
	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(rr.channelName)
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			log.WithFields(log.Fields{
				"channel": v.Channel,
				"message": string(v.Data),
			}).Println("Redis Message Received")
			msg, err := validateMessage(v.Data) //TODO: this shouldn't be here
			if err != nil {
				log.WithFields(log.Fields{
					"err":  err,
					"data": v.Data,
					"msg":  msg,
				}).Error("Error unmarshalling message from Redis")
				continue
			}
			rr.broadcast(v.Data)
		case redis.Subscription:
			log.WithFields(log.Fields{
				"channel": v.Channel,
				"kind":    v.Kind,
				"count":   v.Count,
			}).Println("Redis Subscription Received")
		case error:
			log.WithField("err", v).Errorf("Error while subscribed to Redis channel %s", rr.channelName)
		default:
			log.WithField("v", v).Println("Unknown Redis receive during subscription")
		}
	}
}

// broadcast the provided message to all connected websocket connections.
// If an error occurs while writting a message to a websocket connection it is
// closed and deregistered.
func (rr *redisReceiver) broadcast(data []byte) {
	rr.Mutex.Lock()
	defer rr.Mutex.Unlock()
	for id, conn := range rr.conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.WithFields(log.Fields{
				"id":   id,
				"data": data,
				"err":  err,
				"conn": conn,
			}).Error("Error writting data to connection! Closing and removing Connection")
			rr.deRegister(id)  //is this a dead-lock? could we defer this instead?
		}
	}
}

// register the websocket connection with the receiver and return a unique
// identifier for the connection. This identifier can be used to deregister the
// connection later
func (rr *redisReceiver) register(conn *websocket.Conn) string {
	rr.Mutex.Lock()
	defer rr.Mutex.Unlock()
	id := uuid.New()
	rr.conns[id] = conn
	return id
}

// deRegister the connection by closing it and removing it from our list.
func (rr *redisReceiver) deRegister(id string) {
	rr.Mutex.Lock()
	defer rr.Mutex.Unlock()
	if conn, ok := rr.conns[id]; ok {
		conn.Close()
		delete(rr.conns, id)
	}
}
