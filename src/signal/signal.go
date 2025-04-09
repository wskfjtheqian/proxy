package signal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"webrtc_proxy/src/channel"
)

type SignalOption func(*Signal)

func NewSignal(options ...SignalOption) *Signal {
	ret := &Signal{
		mux:         http.NewServeMux(),
		usernameMap: make(map[string]string),
		wsMap:       make(map[string]*WebSocket),
		wsLock:      sync.RWMutex{},
	}

	for _, option := range options {
		option(ret)
	}

	ret.mux.HandleFunc("/websocket/signal", ret.websocketHandler)
	ret.mux.HandleFunc("/http/signal", ret.httpHandler)
	return ret
}

type Signal struct {
	server      *http.Server
	mux         *http.ServeMux
	addr        string
	usernameMap map[string]string
	wsMap       map[string]*WebSocket
	wsLock      sync.RWMutex
}

func WithAddr(addr string) SignalOption {
	return func(s *Signal) {
		s.addr = addr
	}
}

func WithUserPass(usernameMap map[string]string) SignalOption {
	return func(s *Signal) {
		s.usernameMap = usernameMap
	}
}

func (s *Signal) Start() {

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}
	go func() {
		log.Println("Starting signal server on addr " + s.addr)
		if err := s.server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
}

func (s Signal) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Signal) websocketHandler(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	username := query.Get("username")
	password := query.Get("password")
	deviceId := query.Get("deviceId")

	if username == "" || deviceId == "" || password == "" {
		log.Println("Invalid request" + request.URL.String())
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if s.usernameMap[username] != password {
		log.Println("Invalid username or password" + request.URL.String())
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	ws := NewWebSocket(deviceId)
	ws.OnOpen(func() {
		log.Println("New websocket connection from " + request.RemoteAddr + " for device " + deviceId)
		s.wsLock.Lock()
		s.wsMap[deviceId] = ws
		s.wsLock.Unlock()
	})
	ws.OnClose(func() {
		log.Println("Websocket connection closed from " + request.RemoteAddr + " for device " + deviceId)
		s.wsLock.Lock()
		delete(s.wsMap, deviceId)
		s.wsLock.Unlock()
	})
	ws.OnConnect(func(targetDeviceID string) error {
		s.wsLock.RLock()
		ws, ok := s.wsMap[targetDeviceID]
		s.wsLock.RUnlock()
		if !ok {
			return fmt.Errorf("Device %s not found", targetDeviceID)
		}
		return ws.Connect(deviceId)
	})
	ws.OnResponseSignal(func(targetDeviceID string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error) {
		s.wsLock.RLock()
		ws, ok := s.wsMap[targetDeviceID]
		s.wsLock.RUnlock()
		if !ok {
			return nil, fmt.Errorf("Device %s not found", targetDeviceID)
		}
		return ws.RequestSignal(deviceId, candidates)
	})
	ws.ServeHTTP(writer, request)
}

func (s *Signal) httpHandler(writer http.ResponseWriter, request *http.Request) {

}
