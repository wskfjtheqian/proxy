package test

import (
	"io"
	"net/http"
	"testing"
	"time"
)

var testData = make([]byte, 1024*1024)

func init() {
	for i := 0; i < len(testData); i++ {
		testData[i] = byte(i % 256)
	}
}

func TestServer(t *testing.T) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 1024; i++ {
			w.Write(testData)
		}
	})

	http.ListenAndServe(":18080", nil)
}

func TestProxy(t *testing.T) {
	client := &http.Client{}
	resp, err := client.Get("http://localhost:8080/")
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()

	//下载文件并显示下载速度
	fileSize := int64(len(testData) * 1024)
	downloadedSize := int64(0)
	startTime := time.Now()
	for {
		buf := make([]byte, 1024*1024)
		n, err := resp.Body.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Error(err)
		}
		downloadedSize += int64(n)
		if downloadedSize > fileSize {
			t.Errorf("downloaded size %d is greater than file size %d", downloadedSize, fileSize)
			return
		}
		if time.Now().Sub(startTime) > time.Second {
			speed := float64(downloadedSize) / time.Now().Sub(startTime).Seconds()
			t.Logf("download speed: %.2f MB/s", speed/1024/1024)
			startTime = time.Now()
		}
	}
	if downloadedSize != fileSize {
		t.Errorf("downloaded size %d is not equal to file size %d", downloadedSize, fileSize)
	}
}
