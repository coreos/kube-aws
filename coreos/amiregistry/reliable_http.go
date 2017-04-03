package amiregistry

import (
	"fmt"
	"log"
	"net/http"
)

const MaxRetryCount = 2

type reliableHttp interface {
	Get(url string) (resp *http.Response, err error)
}

type reliableHttpImpl struct {
	underlyingGet func(url string) (resp *http.Response, err error)
}

func newHttp() reliableHttpImpl {
	return reliableHttpImpl{
		underlyingGet: http.Get,
	}
}

func (i reliableHttpImpl) Get(url string) (resp *http.Response, err error) {
	var r *http.Response
	var e error

	c := 0
	for c <= MaxRetryCount {
		c++
		r, e = i.underlyingGet(url)

		if r != nil && r.StatusCode == 504 {
			log.Printf("GET %s returned %d. retrying %d/%d", url, r.StatusCode, c, MaxRetryCount)
		} else if e == nil {
			return r, nil
		}

		if e != nil {
			log.Printf("GET %s failed due to \"%v\". retrying %d/%d", url, e, c, MaxRetryCount)
		}
	}

	return r, fmt.Errorf("max retry count exceeded: %v", e)
}
