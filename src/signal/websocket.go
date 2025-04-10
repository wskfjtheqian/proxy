package signal

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	"webrtc_proxy/src/channel"
)

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 65536,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Message struct {
	Type   string          `json:"type"`
	Id     uint64          `json:"id"`
	Event  string          `json:"event"`
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

type WebSocket struct {
	conn             *websocket.Conn
	onOpen           func()
	onClose          func()
	writeLock        sync.Mutex
	onConnect        func(deviceId string) error
	onResponseSignal func(deviceId string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error)
	deviceId         string

	id           atomic.Uint64
	responseMap  map[uint64]chan *Message
	responseLock sync.RWMutex
}

func NewWebSocket(deviceId string) *WebSocket {
	ret := &WebSocket{
		deviceId:     deviceId,
		responseMap:  make(map[uint64]chan *Message),
		responseLock: sync.RWMutex{},
	}
	return ret
}

func (w *WebSocket) Dial(addr string) error {
	//parse, err := url.Parse(addr)
	//if err != nil {
	//	return err
	//}
	//origin := parse.Scheme + "://" + parse.Host
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(addr, nil)
	if err != nil {
		return err
	}
	w.conn = conn
	go func() {
		err := w.read()
		if err != nil {
			log.Println(err)
		}
	}()
	return nil
}

func (w *WebSocket) Close() error {
	return w.conn.Close()
}

func (w *WebSocket) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	conn, err := upGrader.Upgrade(writer, request, nil)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.conn = conn
	err = w.read()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (w *WebSocket) read() error {
	if w.onOpen != nil {
		go w.onOpen()
	}
	defer func() {
		if w.onClose != nil {
			w.onClose()
		}
	}()
	for {
		msg := Message{}
		err := w.conn.ReadJSON(&msg)
		if err != nil {
			return err
		}
		go func() {
			err = w.onMessage(msg)
			if err != nil {
				log.Println(err)
			}
		}()

	}
}

type RequestSignal struct {
	DeviceId   string                          `json:"device_id"`
	Candidates channel.SessionAndICECandidates `json:"candidates"`
}

type RequestConnect struct {
	DeviceId string `json:"device_id"`
}

func (w *WebSocket) RequestSignal(deviceId string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error) {
	data, err := json.Marshal(&RequestSignal{
		DeviceId:   deviceId,
		Candidates: *candidates,
	})
	if err != nil {
		return nil, err
	}

	responseData, err := w.writeRequest("send-signal", data)
	if err != nil {
		return nil, err
	}

	var response channel.SessionAndICECandidates
	err = json.Unmarshal(responseData, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (w *WebSocket) OnResponseSignal(response func(deviceId string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error)) {
	w.onResponseSignal = response
}

func (w *WebSocket) OnConnect(f func(deviceId string) error) {
	w.onConnect = f
}

func (w *WebSocket) OnOpen(f func()) {
	w.onOpen = f
}

func (w *WebSocket) OnClose(f func()) {
	w.onClose = f
}

func (w *WebSocket) Connect(deviceId string) error {
	data, err := json.Marshal(RequestConnect{
		DeviceId: deviceId,
	})
	if err != nil {
		return err
	}

	_, err = w.writeRequest("connect", data)
	if err != nil {
		return err
	}
	return nil
}

func (w *WebSocket) onMessage(msg Message) error {
	if msg.Type == "request" {
		switch msg.Event {
		case "connect":
			var request RequestConnect
			err := json.Unmarshal(msg.Data, &request)
			if err != nil {
				return w.writeResponse(msg.Id, msg.Event, nil, err)
			}
			err = w.onConnect(request.DeviceId)
			if err != nil {
				return w.writeResponse(msg.Id, msg.Event, nil, err)
			}
			return w.writeResponse(msg.Id, msg.Event, nil, nil)
		case "send-signal":
			var request RequestSignal
			err := json.Unmarshal(msg.Data, &request)
			if err != nil {
				return w.writeResponse(msg.Id, msg.Event, nil, err)
			}
			response, err := w.onResponseSignal(request.DeviceId, &request.Candidates)
			if err != nil {
				return w.writeResponse(msg.Id, msg.Event, nil, err)
			}
			data, err := json.Marshal(response)
			if err != nil {
				return w.writeResponse(msg.Id, msg.Event, nil, err)
			}
			return w.writeResponse(msg.Id, msg.Event, data, nil)
		}
	} else if msg.Type == "response" {
		w.responseLock.RLock()
		ch, ok := w.responseMap[msg.Id]
		if ok {
			ch <- &msg
		}
		w.responseLock.RUnlock()
	}
	return nil
}

func (w *WebSocket) writeRequest(event string, data []byte) (json.RawMessage, error) {
	id := w.id.Add(1)
	msg := Message{
		Type:  "request",
		Event: event,
		Id:    id,
		Data:  json.RawMessage(data),
	}

	w.responseLock.Lock()
	w.responseMap[id] = make(chan *Message)
	w.responseLock.Unlock()

	defer func() {
		w.responseLock.Lock()
		close(w.responseMap[id])
		delete(w.responseMap, id)
		w.responseLock.Unlock()
	}()

	err := func() error {
		w.writeLock.Lock()
		defer w.writeLock.Unlock()
		return w.conn.WriteJSON(&msg)
	}()
	if err != nil {
		return nil, err
	}

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	select {
	case response := <-w.responseMap[id]:
		if response.Status != "ok" {
			return nil, errors.New(response.Status)
		}
		return response.Data, nil
	case <-timer.C:
		return nil, errors.New("timeout")
	}
}

func (w *WebSocket) writeResponse(id uint64, event string, data json.RawMessage, err error) error {
	msg := Message{
		Type:   "response",
		Event:  event,
		Id:     id,
		Status: "ok",
		Data:   data,
	}
	if err != nil {
		msg.Status = err.Error()
	}

	w.writeLock.Lock()
	defer w.writeLock.Unlock()

	err = w.conn.WriteJSON(&msg)
	if err != nil {
		return err
	}
	return nil
}
