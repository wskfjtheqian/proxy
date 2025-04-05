package test

import (
	"io/ioutil"
	"net/http"
	"testing"
)

var testData = make([]byte, 1024*1024)

func init() {
	for i := 0; i < len(testData); i++ {
		testData[i] = byte(i % 256)
	}
}

func TestServer(t *testing.T) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(testData)
	})

	http.ListenAndServe(":18080", nil)
}

func TestClient(t *testing.T) {
	client := &http.Client{}
	resp, err := client.Get("http://localhost:18080/")
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}
	if string(body) != "Hello, world!" {
		t.Errorf("got %s", string(body))
	}
}

func TestProxy(t *testing.T) {
	client := &http.Client{}
	resp, err := client.Get("http://localhost:8080/")
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}
	for i := 0; i < len(testData); i++ {
		if body[i] != testData[i] {
			t.Errorf("got %d, want %d", body[i], testData[i])
			return
		}
	}
}
