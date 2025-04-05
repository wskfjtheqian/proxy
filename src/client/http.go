package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"webrtc_proxy/src/public"
)

type Http struct {
	baseUrl string
}

type Option func(*Http)

func WithUrl(url string) Option {
	return func(h *Http) {
		h.baseUrl = url
	}
}

func NewHttp(options ...Option) *Http {
	ret := &Http{
		baseUrl: "http://127.0.0.1:9090",
	}
	for _, option := range options {
		option(ret)
	}
	return ret
}

func (h *Http) SendOfferAndICECandidates(oic public.OfferAndICECandidates) (*public.AnswerAndICECandidates, error) {
	body := bytes.NewBuffer(nil)
	jsonEncoder := json.NewEncoder(body)
	if err := jsonEncoder.Encode(oic); err != nil {
		return nil, err
	}
	resp, err := http.Post(h.baseUrl+"/offer", "application/json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var answer public.AnswerAndICECandidates
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, err
	}
	return &answer, nil
}
