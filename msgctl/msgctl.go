// Package msgctl reduces messages within a specified interval into one.
package msgctl

import "time"

type MsgController struct {
	In       chan struct{}
	Out      chan struct{}
	interval time.Duration
}

func New(interval time.Duration) *MsgController {
	mc := &MsgController{
		In:       make(chan struct{}),
		Out:      make(chan struct{}),
		interval: interval,
	}
	go mc.run()
	return mc
}

func (mc *MsgController) run() {
	var msg []struct{}
	ticker := time.Tick(mc.interval)
	for {
		select {
		case <-mc.In:
			msg = append(msg, struct{}{})
		case <-ticker:
			if len(msg) > 0 {
				mc.Out <- struct{}{}
				msg = nil
			}
		}
	}
}

// TODO
