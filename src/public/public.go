package public

import "github.com/pion/webrtc/v3"

type OfferAndICECandidates struct {
	Offer         webrtc.SessionDescription `json:"offer"`
	ICECandidates []webrtc.ICECandidateInit `json:"ice_candidates"`
}

type AnswerAndICECandidates struct {
	Answer        webrtc.SessionDescription `json:"answer"`
	ICECandidates []webrtc.ICECandidateInit `json:"ice_candidates"`
}

type ProxyContent interface {
	Open() error
	Close() error
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
}

const MessageTypeOpenRequest = 0xFF102201
const MessageTypeOpenResponse = 0xFF202201

const MessageTypeCloseRequest = 0xFF102202
const MessageTypeCloseResponse = 0xFF202202

const MessageTypeDataRequest = 0xFF102203
const MessageTypeDataResponse = 0xFF202203
