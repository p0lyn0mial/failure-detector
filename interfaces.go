package failure_detector

import (
	"fmt"
	"net/url"
	"time"
)

type KeyFunc func(obj interface{}) string
type NewStoreFunc func(keyFn KeyFunc, ttl time.Duration) Store

type Store interface {
	Add(obj interface{})
	Get(key string) interface{}
}

type wrappedStore struct {
	Store
	batchKey string
}

func newWrappedStore(delegate Store, batchKey string) *wrappedStore {
	return &wrappedStore{Store: delegate, batchKey:batchKey}
}

type BatchQueue interface {
    Get() (key string, items []interface{})
	Add(item interface{})
	Done(key string)
}

// EndpointSample this is a struct that will be collected by the aggregator
type EndpointSample struct {
	namespace string
	service string
	url *url.URL
	err error
}

func EndpointSampleToServiceKeyFunction(obj interface{}) string {
	item := obj.(*EndpointSample)
	return  fmt.Sprintf("%s/%s", item.namespace, item.service)
}

func EndpointSampleKeyFunction(obj interface{}) string {
	item := obj.(*EndpointSample)
	if item.url == nil {
		return ""
	}
	return item.url.Host
}
func wrappedStoreKeyFunction(obj interface{}) string {
	s := obj.(*wrappedStore)
	return  s.batchKey
}

func endpointKeyFunction(obj interface{}) string {
	item := obj.(*Endpoint)
	if item.url == nil {
		return ""
	}
	return item.url.Host
}

// Endpoint a struct that will be stored by the detector.
// It will be examined by the external policy to determine the current status (success/failure)
type Endpoint struct {
    data []*Sample
	position int

    url *url.URL
	status string
	weight float32
}

type Sample struct{
	err error
	// TODO: store latency
}

func newEndpoint(size int, url *url.URL) *Endpoint {
	ep := &Endpoint{}
	ep.data = make([]*Sample, size, size)
	ep.url = url
	return ep
}

func(ep *Endpoint) Add(sample *Sample) {
	size := cap(ep.data)
	ep.position = ep.position % size
	ep.data[ep.position] = sample
	ep.position = ep.position + 1
}

func (ep *Endpoint) Get() []*Sample {
	size := cap(ep.data)
	ret := []*Sample{}

	for i := ep.position % size; i < size; i++ {
		if ep.data[i] == nil {
			break
		}
		ret = append(ret, ep.data[i])
	}
	for i := 0; i < ep.position%size; i++ {
		ret = append(ret, ep.data[i])
	}

	return ret
}
