package epcache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/EndlessParadox1/epcache/consistenthash"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"google.golang.org/protobuf/proto"
)

const (
	defaultBasePath = "/epcache/"
	defaultReplicas = 50
)

type httpGetter struct {
	baseURL string
}

func (hg *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	url_ := fmt.Sprintf("%s%s/%s", hg.baseURL, url.QueryEscape(in.GetGroup()), url.QueryEscape(in.GetKey()))
	res, err := http.Get(url_)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %s", res.Status)
	}
	bytes, err_ := io.ReadAll(res.Body) // bytes encoded by protobuf
	if err_ != nil {
		return fmt.Errorf("reading response body: %s", err.Error())
	}
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

type HTTPPool struct {
	self        string
	basePath    string
	mu          sync.RWMutex
	peers       *consistenthash.Map
	httpGetters map[string]*httpGetter // key like 'http://192.168.0.3:8080'
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (hp *HTTPPool) Log(format string, a ...any) {
	log.Printf("[Server %s] %s\n", hp.self, fmt.Sprintf(format, a...))
}

// Set reset the pool's list of peers, including self
func (hp *HTTPPool) Set(peers ...string) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.peers = consistenthash.New(defaultReplicas, nil)
	hp.peers.Add(peers...)
	hp.httpGetters = make(map[string]*httpGetter)
	for _, peer := range peers {
		hp.httpGetters[peer] = &httpGetter{baseURL: peer + hp.basePath}
	}
}

// PickPeer picks a peer according to the key
func (hp *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	if peer := hp.peers.Get(key); peer != "" && peer != hp.self {
		hp.Log("Pick peer: %s", peer)
		return hp.httpGetters[peer], true
	}
	return nil, false
}

func (hp *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, hp.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	hp.Log("%s %s", r.Method, r.URL.Path)
	parts := strings.SplitN(r.URL.Path[len(hp.basePath):], "/", 2)
	name := parts[0]
	key := parts[1]
	group := GetGroup(name)
	if group == nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err_ := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err_ != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}
