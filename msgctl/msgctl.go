// Package msgctl reduces messages within a specified interval into one.
package msgctl

import "time"

type MsgController struct {
	in       chan struct{}
	out      chan struct{}
	interval time.Duration
	clsCh    chan struct{}
}

func New(interval time.Duration) *MsgController {
	mc := &MsgController{
		in:       make(chan struct{}),
		out:      make(chan struct{}),
		interval: interval,
		clsCh:    make(chan struct{}),
	}
	go mc.run()
	return mc
}

func (mc *MsgController) Send() {
	mc.in <- struct{}{}
}

func (mc *MsgController) Recv() <-chan struct{} {
	return mc.out
}

func (mc *MsgController) Close() {
	close(mc.clsCh)
}

func (mc *MsgController) run() {
	var msg []struct{}
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-mc.in:
			msg = append(msg, struct{}{})
		case <-ticker.C:
			if len(msg) > 0 {
				mc.out <- struct{}{}
				msg = nil
			}
		case <-mc.clsCh:
			return
		}
	}
}
