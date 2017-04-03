package amiregistry

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
)

type unreliableTransportHttpGetter struct {
	numSimulatedErrors int
}

func (g *unreliableTransportHttpGetter) Get(url string) (resp *http.Response, err error) {
	if g.numSimulatedErrors > 0 {
		g.numSimulatedErrors--
		return nil, fmt.Errorf("simulated transport error: errors left=%d", g.numSimulatedErrors)
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}, nil
}

type unreliableApplicationHttpGetter struct {
	numSimulatedErrors int
	statusCode         int
}

func (g *unreliableApplicationHttpGetter) Get(url string) (resp *http.Response, err error) {
	if g.statusCode <= 0 {
		log.Panicf("unsupported status code: %d", g.statusCode)
	}

	if g.numSimulatedErrors > 0 {
		g.numSimulatedErrors--
		return &http.Response{
			StatusCode: g.statusCode,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf("simulated application error: errors left=%d", g.numSimulatedErrors)))),
		}, nil
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}, nil
}

func TestAuthTokenGeneration(t *testing.T) {
	t.Run("WithUnreliableTransport", func(t *testing.T) {
		t.Run("SucceedAfterRetries", func(t *testing.T) {
			twoErrors := &unreliableTransportHttpGetter{
				numSimulatedErrors: 2,
			}
			h1 := reliableHttpImpl{
				underlyingGet: twoErrors.Get,
			}
			r1, e1 := h1.Get("http://example.com/status")
			if e1 != nil {
				t.Errorf("expected an reliable http get to succeed after implicit retries, but it didn't: %v", e1)
			}
			if r1.StatusCode != 200 {
				t.Errorf("expected an reliable http get to succeed after implicit retries, but it didn't: invalid status code: %d", r1.StatusCode)
			}
		})

		t.Run("FailAfterRetries", func(t *testing.T) {
			threeErrors := &unreliableTransportHttpGetter{
				numSimulatedErrors: 3,
			}
			h2 := reliableHttpImpl{
				underlyingGet: threeErrors.Get,
			}
			_, e2 := h2.Get("http://example.com/status")
			if e2 == nil {
				t.Error("expected an reliable http get to fail after exceeding max retry count, but it didn't")
			}
		})
	})

	t.Run("WithUnreliableApplication", func(t *testing.T) {
		t.Run("SucceedAfterRetries", func(t *testing.T) {
			twoErrors := &unreliableApplicationHttpGetter{
				numSimulatedErrors: 2,
				statusCode:         504,
			}
			h1 := reliableHttpImpl{
				underlyingGet: twoErrors.Get,
			}
			r, e := h1.Get("http://example.com/status")
			if r == nil {
				t.Error("expected an reliable http get to return non-nil response, but it didn't")
				t.FailNow()
			}
			if r.StatusCode != 200 {
				t.Errorf("expected an reliable http get to succeed after implicit retries, but it didn't: invalid status code \"%d\", error was \"%v\"", r.StatusCode, e)
			}
		})

		t.Run("FailAfterRetries", func(t *testing.T) {
			threeErrors := &unreliableApplicationHttpGetter{
				numSimulatedErrors: 3,
				statusCode:         504,
			}
			h := reliableHttpImpl{
				underlyingGet: threeErrors.Get,
			}
			r, e := h.Get("http://example.com/status")
			if r == nil {
				t.Error("expected an reliable http get to return non-nil response, but it didn't")
				t.FailNow()
			}
			if r.StatusCode != 504 {
				t.Errorf("expected an reliable http get to fail after exceeding max retry count, but it didn't: invalid status code \"%d\", error was \"%v\"", r.StatusCode, e)
			}
		})
	})
}
