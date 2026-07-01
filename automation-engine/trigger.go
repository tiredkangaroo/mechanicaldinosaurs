package main

import (
	"fmt"
	"sync"
	"time"
)

type Trigger interface {
	Type() string
	Register(cb func()) error
	Deregister() error
}

// time trigger
type TimeTrigger struct {
	t         time.Time
	c         chan struct{}
	closeOnce sync.Once
}

func (t *TimeTrigger) Type() string {
	return "time"
}
func (t *TimeTrigger) Register(cb func()) error {
	until := time.Until(t.t)
	if until <= 0 {
		return fmt.Errorf("time trigger: time %v is in the past", t.t)
	}
	go func() {
		timer := time.NewTimer(until)
		defer timer.Stop()
		select {
		case <-timer.C:
			cb()
		case <-t.c:
			return
		}
	}()
	return nil
}
func (t *TimeTrigger) Deregister() error {
	t.closeOnce.Do(func() {
		close(t.c)
	})
	return nil
}

// interval trigger
type IntervalTrigger struct {
	d         time.Duration
	c         chan struct{}
	closeOnce sync.Once
}

func (i *IntervalTrigger) Type() string {
	return "interval"
}
func (i *IntervalTrigger) Register(cb func()) error {
	go func() {
		ticker := time.NewTicker(i.d)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cb()
			case <-i.c:
				return
			}
		}
	}()
	return nil
}
func (i *IntervalTrigger) Deregister() error {
	i.closeOnce.Do(func() {
		close(i.c)
	})
	return nil
}

func NewTimeTrigger(t time.Time) *TimeTrigger {
	return &TimeTrigger{
		t: t,
		c: make(chan struct{}),
	}
}

func NewIntervalTrigger(d time.Duration) *IntervalTrigger {
	return &IntervalTrigger{
		d: d,
		c: make(chan struct{}),
	}
}

// data about machines just got refreshed
type DataRefreshTrigger struct {
	subscription chan struct{}
	c            chan struct{}
	closeOnce    sync.Once
}

func (d *DataRefreshTrigger) Type() string {
	return "data_refresh"
}
func (d *DataRefreshTrigger) Register(cb func()) error {
	go func() {
		for {
			select {
			case <-d.subscription:
				cb()
			case <-d.c:
				return
			}
		}
	}()
	return nil
}
func (d *DataRefreshTrigger) Deregister() error {
	d.closeOnce.Do(func() {
		close(d.c)
		dataRefreshAlerts.unsubscribe(d.subscription)
	})
	return nil
}

func NewDataRefreshTrigger() *DataRefreshTrigger {
	return &DataRefreshTrigger{
		c:            make(chan struct{}),
		subscription: dataRefreshAlerts.subscribe(),
	}
}
