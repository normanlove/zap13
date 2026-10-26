package main

import (
	"strings"
	"sync"
	"time"
)

const minW, maxW = 5, 120

type limits [3]int // SPL, sPPT, fPPT

// keeps SPL <= sPPT <= fPPT and everything in range
func (l limits) clamp() limits {
	l[0] = clampInt(l[0], minW, maxW)
	l[1] = clampInt(l[1], l[0], maxW)
	l[2] = clampInt(l[2], l[1], maxW)
	return l
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

type controller struct {
	mu     sync.Mutex
	atk    *atk
	active *limits // nil when the firmware is using its own limits
	log    func(string)
}

func (c *controller) apply(l limits, mode uint32) limits {
	l = l.clamp()

	c.mu.Lock()
	c.atk.set(devPerfMode, mode)
	time.Sleep(500 * time.Millisecond) // mode switch resets the limits, let it finish first
	res := c.write(l)
	c.active = &l
	c.mu.Unlock()

	var rejected []string
	for name, r := range res {
		if r != 1 {
			rejected = append(rejected, name)
		}
	}
	if len(rejected) > 0 {
		c.log("Firmware rejected " + strings.Join(rejected, ", ") + " (out of range?)")
	}
	return l
}

func (c *controller) write(l limits) map[string]int32 {
	return map[string]int32{
		"sPPT": c.atk.set(devPPTSlow, uint32(l[1])),
		"SPL":  c.atk.set(devPPTSPL, uint32(l[0])),
		"fPPT": c.atk.set(devPPTFast, uint32(l[2])),
	}
}

// the firmware sometimes puts its own values back, so this gets called every so often
func (c *controller) reapply() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil {
		c.write(*c.active)
	}
}

func (c *controller) setMode(mode uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active != nil {
		// switching away and back makes the firmware reload its default limits
		other := uint32(modeSilent)
		if mode == modeSilent {
			other = modeBalanced
		}
		c.atk.set(devPerfMode, other)
		time.Sleep(300 * time.Millisecond)
	}
	c.atk.set(devPerfMode, mode)
	c.active = nil
}

func (c *controller) current() *limits {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil {
		return nil
	}
	l := *c.active
	return &l
}

func (c *controller) chargerType(onAC bool) string {
	if !onAC {
		return "battery"
	}
	c.mu.Lock()
	v, err := c.atk.get(devCharger)
	c.mu.Unlock()
	switch {
	case err != nil || v < 0:
		return "ac" // plugged in but couldn't tell which port
	case v&1 != 0:
		return "barrel"
	case v&2 != 0:
		return "usbc"
	}
	return "ac"
}
