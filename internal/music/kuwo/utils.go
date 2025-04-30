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
