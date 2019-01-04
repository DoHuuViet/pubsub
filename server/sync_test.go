package main

import (
	"net/http"
	"sync"
	"testing"
)

func Test_Channel_SendNilToNobody(t *testing.T) {

	channel := NewChannel()

	channel.Send("not-found", nil)
}

func Test_Channel_SendResponseToNobody(t *testing.T) {

	channel := NewChannel()

	channel.Send("not-found", &http.Response{})
}

func Test_Channel_SubscriptionList(t *testing.T) {

	channel := NewChannel()
	list := channel.SubscriptionList()
	if len(list) != 0 {
		t.Errorf("want 0 items, got %d", len(list))
	}

	channel.Subscribe("1234")
	list = channel.SubscriptionList()
	if len(list) != 1 {
		t.Errorf("want 1 item, got %d", len(list))
	}
}

func Test_Channel_SendResponseToOneReceiver(t *testing.T) {
	channel := NewChannel()
	sub := channel.Subscribe("1234")
	wg := sync.WaitGroup{}
	wg.Add(1)

	var res *http.Response
	go func() {
		res = <-sub.Data

		wg.Done()
		return
	}()

	response := &http.Response{Header: http.Header{}}
	response.Header.Set("inlets-id", "1234")
	channel.Send("1234", response)
	wg.Wait()

	if res.Header.Get("inlets-id") != "1234" {
		t.Errorf("want header: 1234. got: %s", res.Header.Get("inlets-id"))
	}
}

func Test_Channel_SendResponseToOneReceiverThenUnsubscribe(t *testing.T) {
	channel := NewChannel()
	sub := channel.Subscribe("1234")
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		<-sub.Data
		channel.Unsubscribe("1234")

		wg.Done()
		return
	}()

	response := &http.Response{Header: http.Header{}}
	response.Header.Set("inlets-id", "1234")
	channel.Send("1234", response)
	channel.Send("1234", response)

	wg.Wait()
}
