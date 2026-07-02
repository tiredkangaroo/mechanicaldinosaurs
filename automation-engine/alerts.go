package main

import "sync"

type Alerts[T any] struct {
	mu        sync.Mutex
	listeners map[chan T]struct{}
}

func (a *Alerts[T]) Broadcast(v T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ch := range a.listeners {
		select {
		case ch <- v:
		default:
			// defaults if a specific listener's buffer is full, we will skip bc we don't want block
			// probably only full if broadcast is called twice and listener hasn't read the first one yet or smth
		}
	}
}

func (a *Alerts[T]) subscribe() chan T {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listeners == nil {
		a.listeners = make(map[chan T]struct{})
	}
	ch := make(chan T, 1)
	a.listeners[ch] = struct{}{}
	return ch
}

func (a *Alerts[T]) unsubscribe(ch chan T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.listeners, ch)
}
