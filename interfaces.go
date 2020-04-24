package failure_detector

import (
	"fmt"
	"net/url"
	"time"
)

type KeyFunc func(obj interface{}) string
type NewStoreFunc func(ttl time.Duration) EndpointStore
type EvaluateFunc func(endpoint *Endpoint) bool

type Store interface {
	Add(key string, obj interface{})
	Get(key string) interface{}
}

type EndpointStore interface {
	Add(key string, obj *Endpoint)
	Get(key string) *Endpoint
}

type endpointStore struct {
	Store
}

func newEndpointStore(delegate Store) *endpointStore {
	return &endpointStore{Store: delegate}
}

func (e *endpointStore) Add(key string, ep *Endpoint) {
	e.Store.Add(key, ep)
}

func (e *endpointStore) Get(key string) *Endpoint {
	rawEndpoint := e.Store.Get(key)
	if rawEndpoint == nil {
		return nil
	}
	return rawEndpoint.(*Endpoint)
}

// TODO: for now - once we move the implementation to this pkg
// we will use only endPointSampleBatchQueue
type BatchQueue interface {
	Get() (key string, items []interface{})
	Add(key string, item interface{})
	Done(key string)
}

type endPointSampleBatchQueue interface {
	Get() (key string, items []*EndpointSample)
	Add(key string, item *EndpointSample)
	Done(key string)
}

type endpointSampleBatchQueue struct {
	BatchQueue
}

func (q *endpointSampleBatchQueue) Get() (key string, items []*EndpointSample) {
	key, rawEndpointSample := q.BatchQueue.Get()
	endpointSamples := make([]*EndpointSample, len(rawEndpointSample))
	for i, r := range rawEndpointSample {
		endpointSamples[i] = r.(*EndpointSample)
	}
	return key, endpointSamples
}

func (q *endpointSampleBatchQueue) Add(key string, item *EndpointSample) {
	q.BatchQueue.Add(key, item)
}

func (q *endpointSampleBatchQueue) Done(key string) {
	q.BatchQueue.Done(key)
}

func newEndPointSampleBatchQueue(delegate BatchQueue) endPointSampleBatchQueue {
	return &endpointSampleBatchQueue{BatchQueue: delegate}
}

// EndpointSample this is a struct that will be collected by the aggregator
type EndpointSample struct {
	namespace string
	service   string
	url       *url.URL
	err       error
}

func EndpointSampleToServiceKeyFunction(obj interface{}) string {
	item := obj.(*EndpointSample)
	return fmt.Sprintf("%s/%s", item.namespace, item.service)
}

func EndpointSampleKeyFunction(obj interface{}) string {
	item := obj.(*EndpointSample)
	if item.url == nil {
		return ""
	}
	return item.url.Host
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
	data     []*Sample
	position int

	url    *url.URL
	status string
	weight float32
}

type Sample struct {
	err error
	// TODO: store latency
}

func newEndpoint(size int, url *url.URL) *Endpoint {
	ep := &Endpoint{}
	ep.data = make([]*Sample, size, size)
	ep.url = url
	return ep
}

func (ep *Endpoint) Add(sample *Sample) {
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
