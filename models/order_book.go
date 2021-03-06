package models

import (
	"time"
)

// Order represents an order
type Order struct {
	Time     time.Time
	Quantity int
	Bid      float64
}

// OrderBook represents the current state of the market
type OrderBook struct {
	Orders []Order
}

// Len gets the length of the
func (o *OrderBook) Len() int {
	return len(o.Orders)
}

// Less determins if the first value is before the second
func (o *OrderBook) Less(i, j int) bool {
	return o.Orders[i].Time.Before(o.Orders[j].Time)
}

// Swap will swap the placement of the elements by their indexes
func (o *OrderBook) Swap(i, j int) {
	o.Orders[i], o.Orders[j] = o.Orders[j], o.Orders[i]
}

// NewOrderBook creates a new value of type OrderBook pointer
func NewOrderBook() *OrderBook {
	return &OrderBook{
		Orders: make([]Order, 0),
	}
}
