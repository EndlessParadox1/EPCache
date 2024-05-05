package msgcompress

import "time"

type MsgCompressor struct {
	In    chan struct{}
	Out   chan struct{}
	delay time.Duration
}

func New(delay time.Duration) *MsgCompressor {
	mc := &MsgCompressor{
		In:    make(chan struct{}),
		Out:   make(chan struct{}),
		delay: delay,
	}
	go mc.run()
	return mc
}

func (mc *MsgCompressor) run() {
	var msg []struct{}
	ticker := time.NewTicker(mc.delay)
	defer ticker.Stop()
	for {
		select {
		case <-mc.In:
			msg = append(msg, struct{}{})
		case <-ticker.C:
			if len(msg) > 0 {
				mc.Out <- struct{}{}
				msg = nil
			}
		}
	}
}

// TODO
