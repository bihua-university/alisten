package kuwo

import (
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

var (
	httpTimeout = 10 * time.Second
	HttpClient  = http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
				MaxVersion: tls.VersionTLS12,
			},
		},
	}
)

type HttpResp struct {
	Err        error
	Data       []byte
	Localtion  string
	RequestUrl string
	SetCookie  string
}

func HttpGetWithHeader(api string, h map[string]string) HttpResp {

	// api = "http://nmobi.kuwo.cn/mobi.s?f=kuwo&q=QTTCEVWADWjGHNKyqOt6peSJECe9IlwYOThEXM42tOPVu6boc62uWnhTsSmlQDn46NvDv+yKU0JVRFu8k+uReJLGA0BF5mBYu2iIKCWTWoRUmBZbIhiYmGiFA4VBFQxTYkMBmqrM6z3Y5Dv+PlOhaTSKohr6nLrrpcwj+9uutB3eZ+rwhGxxUTDr4X9EK/thXu7we7ZPNj4d1DNod+PNHN/dhLfP8Bb+vN5Wm7nYA5652W77PsEkq0AyTjnXztdYpv2gWTB2SyIRwzfRMnIiU2yxs2+btb06Y90o3XTg2o+1E92itYrzMA=="

	var result HttpResp
	request, err := http.NewRequest("GET", api, nil)
	if err != nil {
		result.Err = err
		return result
	}

	for k, v := range h {
		request.Header[k] = []string{v}
		//request.Header.Set(k, v)
	}
	request.URL.Query()
	// q := request.URL.Query()
	// request.URL.RawQuery = q.Encode()
	resp, err := HttpClient.Do(request)
	if err != nil {
		result.Err = err
		return result
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, _ := io.ReadAll(resp.Body)
	result.Data = body
	result.RequestUrl = resp.Request.URL.String()
	if resp.Header != nil {
		setCk := resp.Header.Get("set-cookie")
		result.SetCookie = setCk
	}
	return result
}
