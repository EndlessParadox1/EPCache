package epcache

import "time"

var nAlive = 0

func (g *Group) alive(key string) {
	g.populateCache(key, ByteView{}, &g.mainCache)
	nAlive++
	<-time.After(15 * time.Minute)
	g.remove(key)
	nAlive--
}
