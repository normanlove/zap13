package main

import (
	"fmt"
	"sync"
	"time"
)

var (
	usbcLimits    = limits{22, 26, 30}
	batteryLimits = limits{15, 18, 22}
)

// auto watches the power source and applies the matching profile.
// Manual limits stay on until "back to auto" or the charger changes.
type auto struct {
	ctl *controller

	mu        sync.Mutex
	manual    *limits
	dirty     bool
	source    string
	lastApply time.Time
	last      *status
	lastLog   string

	wake     chan struct{}
	onStatus func(status)
	onLog    func(string)
}

func newAuto(a *atk) *auto {
	au := &auto{wake: make(chan struct{}, 1), dirty: true}
	au.ctl = &controller{atk: a, log: au.log}
	return au
}

func (a *auto) log(msg string) {
	line := time.Now().Format("15:04") + "  " + msg
	a.mu.Lock()
	a.lastLog = line
	a.mu.Unlock()
	if a.onLog != nil {
		a.onLog(line)
	}
}

func (a *auto) kick() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func (a *auto) setManual(l limits) {
	l = l.clamp()
	a.mu.Lock()
	a.manual, a.dirty = &l, true
	a.mu.Unlock()
	a.kick()
}

func (a *auto) backToAuto() {
	a.mu.Lock()
	a.manual, a.dirty = nil, true
	a.mu.Unlock()
	a.kick()
}

func (a *auto) applyFor(src string, manual *limits) {
	switch {
	case manual != nil:
		l := a.ctl.apply(*manual, modeTurbo)
		a.log(fmt.Sprintf("Manual limits %d / %d / %d W", l[0], l[1], l[2]))
	case src == "barrel":
		a.ctl.setMode(modeTurbo)
		a.log("Mains adapter - max performance")
	case src == "battery":
		a.ctl.apply(batteryLimits, modeSilent)
		a.log("Battery - lowest power")
	default:
		a.ctl.apply(usbcLimits, modeTurbo)
		a.log("USB-C - Type-C gaming mode")
	}
	a.lastApply = time.Now()
}

func (a *auto) run() {
	for {
		st := readPower()
		src := a.ctl.chargerType(st.AC)

		a.mu.Lock()
		changed := a.source != "" && src != a.source
		dropManual := changed && a.manual != nil
		if dropManual {
			a.manual = nil
		}
		apply := a.dirty || src != a.source
		a.dirty = false
		manual := a.manual
		a.mu.Unlock()

		if dropManual {
			a.log("Power source changed - back to auto")
		}

		if apply {
			a.applyFor(src, manual)
		} else if a.ctl.current() != nil && time.Since(a.lastApply) > 30*time.Second {
			a.ctl.reapply()
			a.lastApply = time.Now()
		}

		st.Source = src
		st.Limits = a.ctl.current()
		a.mu.Lock()
		a.source = src
		st.Manual = a.manual != nil
		a.last = &st
		a.mu.Unlock()

		if a.onStatus != nil {
			a.onStatus(st)
		}

		select {
		case <-a.wake:
		case <-time.After(5 * time.Second):
		}
	}
}
