package server

import (
	"encoding/json"
	"log"
	"net/http"
	"webrtc_proxy/src/public"
)

type Option func(*Http)

type Http struct {
	addr string
}

func WithAddr(addr string) Option {
	return func(h *Http) {
		h.addr = addr
	}
}

func NewHttp(option ...Option) *Http {
	ret := &Http{
		addr: ":9090",
	}

	for _, o := range option {
		o(ret)
	}
	return ret
}

func (h *Http) offerHandler(writer http.ResponseWriter, request *http.Request) {
	var oic public.OfferAndICECandidates
	err := json.NewDecoder(request.Body).Decode(&oic)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	s := NewRTC()
	aic, err := s.OnOfferAndIceCandidate(oic)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	err = json.NewEncoder(writer).Encode(aic)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (h *Http) Start() error {
	http.HandleFunc("/offer", h.offerHandler)
	log.Println("Listening on " + h.addr)
	return http.ListenAndServe(h.addr, nil)
}
