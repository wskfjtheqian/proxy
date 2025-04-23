package signal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"webrtc_proxy/src/channel"
)

type HTTP struct {
	url              *url.URL
	client           *http.Client
	deviceId         string
	onResponseSignal func(targetDeviceID string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error)
}

func NewHTTP(deviceId string) *HTTP {
	return &HTTP{
		client:   &http.Client{},
		deviceId: deviceId,
	}
}

func (h *HTTP) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	req := &channel.SessionAndICECandidates{}
	err := json.NewDecoder(request.Body).Decode(req)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	response, err := h.onResponseSignal(h.deviceId, req)
	if err != nil {
		return
	}
	body := &bytes.Buffer{}
	err = json.NewEncoder(body).Encode(response)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = writer.Write(body.Bytes())

	writer.WriteHeader(http.StatusOK)
}

func (h *HTTP) SetUrl(url *url.URL) {
	h.url = url
}

func (h *HTTP) RequestSignal(deviceId string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error) {
	u := *h.url

	u.Path = path.Join(u.Path, deviceId)
	req := &bytes.Buffer{}
	err := json.NewEncoder(req).Encode(candidates)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", h.url.String(), req)
	if err != nil {
		return nil, err
	}
	response, err := h.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var aic channel.SessionAndICECandidates
	err = json.NewDecoder(response.Body).Decode(&aic)
	if err != nil {
		return nil, err
	}
	return &aic, nil
}

func (h *HTTP) OnResponseSignal(response func(targetDeviceID string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error)) {
	h.onResponseSignal = response
}
