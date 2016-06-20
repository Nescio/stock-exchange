package main

import (
	"net/url"
	"time"

	"github.com/garyburd/redigo/redis"
)

func newRedisPool(us string) (*redis.Pool, error) {
	u, err := url.Parse(us)
	if err != nil {
		return nil, err
	}

	var password string
	if u.User != nil {
		password, _ = u.User.Password()
	}
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", u.Host)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}, nil
}