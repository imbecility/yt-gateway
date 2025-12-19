package client

import (
	"fmt"
	"net/http"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"github.com/imbecility/yt-gateway/pkg/providers"
)

type tlsWrapper struct {
	innerClient tls_client.HttpClient
}

func (w *tlsWrapper) Do(req *http.Request) (*http.Response, error) {
	fReq := &fhttp.Request{
		Method:        req.Method,
		URL:           req.URL,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Header:        make(fhttp.Header),
		Body:          req.Body,
		ContentLength: req.ContentLength,
		Host:          req.Host,
	}

	for k, v := range req.Header {
		fReq.Header[k] = v
	}

	if c := req.Header.Get("Cookie"); c != "" {
		fReq.Header.Set("Cookie", c)
	}

	resp, err := w.innerClient.Do(fReq)
	if err != nil {
		return nil, err
	}

	netResp := &http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		ContentLength:    resp.ContentLength,
		Body:             resp.Body,
		Header:           make(http.Header),
		Uncompressed:     resp.Uncompressed,
		TransferEncoding: resp.TransferEncoding,
		// Request: req, // сan be linked back
	}

	for k, v := range resp.Header {
		netResp.Header[k] = v
	}

	return netResp, nil
}

func NewHttpClient() (providers.HTTPClient, error) {
	jar := tls_client.NewCookieJar()

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(600), // long videos can take more time to load.
		tls_client.WithClientProfile(profiles.DefaultClientProfile),
		tls_client.WithInsecureSkipVerify(), // some providers have proxied links without https
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithCookieJar(jar),
	}

	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls client: %w", err)
	}

	return &tlsWrapper{innerClient: c}, nil
}
