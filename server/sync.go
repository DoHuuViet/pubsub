package main

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"
)

type Channel struct {
	Subscriptions map[string]Subscription
	Mutex         *sync.RWMutex
}

type Subscription struct {
	Data   chan *http.Response
	Closed *uint32
}

func NewChannel() *Channel {
	return &Channel{
		Subscriptions: make(map[string]Subscription),
		Mutex:         &sync.RWMutex{},
	}
}

func (c *Channel) SubscriptionList() []string {
	keys := []string{}

	c.Mutex.Lock()

	defer c.Mutex.Unlock()

	for key := range c.Subscriptions {
		keys = append(keys, key)
	}
	return keys
}

func (c *Channel) Send(id string, res *http.Response) {
	var ok bool
	var closed bool
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()

	_, ok = c.Subscriptions[id]

	if !ok {
		return
	}

	closed = c.Subscriptions[id].Closed != nil &&
		atomic.LoadUint32(c.Subscriptions[id].Closed) > uint32(0)

	if !closed {
		log.Printf("sub-send: %s=%v", id, c.Subscriptions[id].Data)
		c.Subscriptions[id].Data <- res
	} else {
		// Ignore closed channels
		log.Println("IGNORE closed channel")
	}
}

func (c *Channel) Subscribe(id string) *Subscription {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	closed := uint32(0)
	sub := Subscription{
		Data:   make(chan *http.Response),
		Closed: &closed,
	}

	c.Subscriptions[id] = sub
	log.Printf("sub-create: %s=%v", id, sub.Data)

	return &sub
}

func (c *Channel) Unsubscribe(id string) {

	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	if _, ok := c.Subscriptions[id]; ok {
		atomic.AddUint32(c.Subscriptions[id].Closed, uint32(1))
		log.Printf("sub-remove: %s=%v", id, c.Subscriptions[id].Data)

		ch := c.Subscriptions[id].Data
		delete(c.Subscriptions, id)
		close(ch)
	}
}
