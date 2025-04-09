package signal

import (
	"net/http"
)

type HTTP struct {
}

func NewHTTP() *HTTP {
	return &HTTP{}
}

func (h *HTTP) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	//var oic public.OfferAndICECandidates
	//err := json.NewDecoder(request.Body).Decode(&oic)
	//if err != nil {
	//	writer.WriteHeader(http.StatusInternalServerError)
	//	log.Println(err)
	//	return
	//}
	//
	////s := NewRTC()
	////aic, err := s.OnOfferAndIceCandidate(oic)
	//if err != nil {
	//	writer.WriteHeader(http.StatusInternalServerError)
	//	log.Println(err)
	//	return
	//}
	//err = json.NewEncoder(writer).Encode(aic)
	//if err != nil {
	//	writer.WriteHeader(http.StatusInternalServerError)
	//	log.Println(err)
	//	return
	//}
	writer.WriteHeader(http.StatusOK)
}
