package main

import "sync"

var dataRefreshAlerts = &Alerts{}

type Alerts struct {
	mu        sync.Mutex
	listeners map[chan struct{}]struct{}
}

func (a *Alerts) Broadcast() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ch := range a.listeners {
		select {
		case ch <- struct{}{}:
		default:
			// defaults if a specific listener's buffer is full, we will skip bc we don't want block
			// probably only full if broadcast is called twice and listener hasn't read the first one yet or smth
		}
	}
}

func (a *Alerts) subscribe() chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listeners == nil {
		a.listeners = make(map[chan struct{}]struct{})
	}
	ch := make(chan struct{}, 1)
	a.listeners[ch] = struct{}{}
	return ch
}

func (a *Alerts) unsubscribe(ch chan struct{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.listeners, ch)
}
