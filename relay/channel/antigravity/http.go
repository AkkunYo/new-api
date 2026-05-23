package antigravity

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

var (
	antigravityHTTP11Transport     *http.Transport
	antigravityHTTP11TransportOnce sync.Once
)

func initAntigravityHTTP11Transport() {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	antigravityHTTP11Transport = cloneAntigravityHTTP11Transport(base)
}

func cloneAntigravityHTTP11Transport(base *http.Transport) *http.Transport {
	if base == nil {
		base = &http.Transport{}
	}
	clone := base.Clone()
	clone.ForceAttemptHTTP2 = false
	clone.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	if clone.TLSClientConfig == nil {
		clone.TLSClientConfig = &tls.Config{}
	} else {
		clone.TLSClientConfig = clone.TLSClientConfig.Clone()
	}
	if common.TLSInsecureSkipVerify {
		clone.TLSClientConfig.InsecureSkipVerify = true
	}
	clone.TLSClientConfig.NextProtos = []string{"http/1.1"}
	return clone
}

func newAntigravityHTTPClient(proxyURL string) (*http.Client, error) {
	base, err := service.NewProxyHttpClient(proxyURL)
	if err != nil {
		return nil, err
	}
	if base == nil {
		antigravityHTTP11TransportOnce.Do(initAntigravityHTTP11Transport)
		return &http.Client{Transport: antigravityHTTP11Transport}, nil
	}
	client := *base
	if transport, ok := base.Transport.(*http.Transport); ok {
		client.Transport = cloneAntigravityHTTP11Transport(transport)
	} else if base.Transport == nil {
		antigravityHTTP11TransportOnce.Do(initAntigravityHTTP11Transport)
		client.Transport = antigravityHTTP11Transport
	}
	if common.RelayTimeout > 0 && client.Timeout == 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return &client, nil
}
