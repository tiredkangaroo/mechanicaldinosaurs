package main

import (
	"fmt"
	"sync"
	"time"
)

type Trigger interface {
	Type() string                    // "time", "interval", "machines info refresh"
	Name() string                    // on event: <name>, this is an explainer, not a unique identifier
	Register(cb func(Context)) error // cb will just be a function that calls condition.Evaluate from the overlying automation, and then calls action.Do() if the condition is true
	Deregister() error               // when an automation is removed, this should be called to stop the trigger from firing
}

// time trigger
type TimeTrigger struct {
	Time      time.Time `json:"time"`
	c         chan struct{}
	closeOnce sync.Once
}

func (t *TimeTrigger) Type() string {
	return "time"
}
func (t *TimeTrigger) Name() string {
	return "time is " + t.Time.String()
}
func (t *TimeTrigger) Register(cb func(Context)) error {
	until := time.Until(t.Time)
	if until <= 0 {
		return fmt.Errorf("time trigger: time %v is in the past", t.Time)
	}
	go func() {
		timer := time.NewTimer(until)
		defer timer.Stop()
		select {
		case <-timer.C:
			cb(Context{
				Data: map[string]any{
					"time": t.Time.Unix(),
				},
			})
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

func NewTimeTrigger(t time.Time) *TimeTrigger {
	return &TimeTrigger{
		Time: t,
		c:    make(chan struct{}),
	}
}

// interval trigger
type IntervalTrigger struct {
	Every     time.Duration `json:"every"`
	c         chan struct{}
	closeOnce sync.Once
}

func (i *IntervalTrigger) Type() string {
	return "interval"
}
func (i *IntervalTrigger) Name() string {
	return "every " + i.Every.String()
}
func (i *IntervalTrigger) Register(cb func(Context)) error {
	go func() {
		ticker := time.NewTicker(i.Every)
		defer ticker.Stop()
		count := 0
		for {
			select {
			case <-ticker.C:
				cb(Context{
					Data: map[string]any{
						"interval": i.Every.Seconds(),
						"count":    count,
					},
				})
				count++
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

func NewIntervalTrigger(d time.Duration) *IntervalTrigger {
	return &IntervalTrigger{
		Every: d,
		c:     make(chan struct{}),
	}
}

// data about machines just got refreshed
type MachineInfoRefreshTrigger struct {
	subscription chan map[string]map[string]any
	c            chan struct{}
	closeOnce    sync.Once
}

func (d *MachineInfoRefreshTrigger) Type() string {
	return "machines info refresh"
}
func (d *MachineInfoRefreshTrigger) Name() string {
	return "machines info refresh"
}
func (d *MachineInfoRefreshTrigger) Register(cb func(Context)) error {
	go func() {
		d.subscription = machineInfoRefreshAlerts.subscribe()
		for {
			select {
			case machinesInfo := <-d.subscription:
				cb(Context{
					Data: map[string]any{
						"machines_info": machinesInfo,
					},
				})
			case <-d.c:
				return
			}
		}
	}()
	return nil
}
func (d *MachineInfoRefreshTrigger) Deregister() error {
	d.closeOnce.Do(func() {
		close(d.c)
		machineInfoRefreshAlerts.unsubscribe(d.subscription)
	})
	return nil
}

func NewMachineInfoRefreshTrigger() *MachineInfoRefreshTrigger {
	return &MachineInfoRefreshTrigger{
		c:            make(chan struct{}),
		subscription: nil, // not subscribed yet, will be subscribed in Register() so we don't register with a shit ton of old machine refreshes
	}
}

type TriggerCommunicable struct {
	Type  string  `json:"type"`            // "time", "interval", "machines info refresh"
	Time  int64   `json:"time,omitempty"`  // exists for time trigger
	Every float64 `json:"every,omitempty"` // exists for interval trigger
}
