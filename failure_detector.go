package failure_detector

import (
	"context"
	"time"

	batchqueue "github.com/p0lyn0mial/batch-working-queue"
	ttlstore "github.com/p0lyn0mial/ttl-cache"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
)

// failureDetector
// - exposes a channel via Collector() method that allows for collecting samples.
// - samples are batched by the namespace/service and processed by processBatch() method
// - processBatch() method calls out to external policy function for assessing the endpoints
// - TODO: internal state is replicated to external storage (atomic.Value) for probing via XYZ() method
type failureDetector struct {
	// endpointSampleKeyFn maps collected sample (EndpointSample) for a service to the internal store
	endpointSampleKeyFn KeyFunc

	processor *processor

	// store holds samples per service (namespace/service)
	//
	// it should:
	//  - automatically removed entries (services) that exceed the configured TTL
	//    that would allows us to remove unused/removed services
	store map[string]EndpointStore

	// createStoreFn a helper function for creating an endpoints store
	//
	// it should:
	//  - automatically removed entries (endpoints) that exceed the configured TTL
	//    that would allows us to remove unused/removed endpoints per service
	createStoreFn NewStoreFunc

	// policyEvaluatorFn an external policy function for assessing the endpoints
	policyEvaluatorFn EvaluateFunc
}

func NewDefaultFailureDetector() *failureDetector {
	createNewStoreFn := func(ttl time.Duration) EndpointStore {
		return newEndpointStore(ttlstore.New(ttl, clock.RealClock{}))
	}
	queue := newEndPointSampleBatchQueue(batchqueue.New())
	return newFailureDetector(EndpointSampleToServiceKeyFunction, SimpleEvaluator, createNewStoreFn, queue)
}

func newFailureDetector(endpointSampleKeyKeyFn KeyFunc, policyEvaluator EvaluateFunc, createStoreFn NewStoreFunc, queue endPointSampleBatchQueue) *failureDetector {
	fd := &failureDetector{}
	processor := newProcessor(endpointSampleKeyKeyFn, fd.processBatch, queue)
	fd.processor = processor
	fd.store = map[string]EndpointStore{}
	fd.endpointSampleKeyFn = endpointSampleKeyKeyFn
	fd.createStoreFn = createStoreFn
	fd.policyEvaluatorFn = policyEvaluator
	return fd
}

func (fd *failureDetector) processBatch(endpointSamples []*EndpointSample) {
	if len(endpointSamples) == 0 {
		return
	}
	batchKey := fd.endpointSampleKeyFn(endpointSamples[0])
	endpointsStore := fd.store[batchKey]
	if endpointsStore == nil {
		endpointsStore = fd.createStoreFn(60 * time.Second)
	}

	visitedEndpointsKey := sets.NewString()
	for _, endpointSample := range endpointSamples {
		endpointKey, sample := convertToKeySample(endpointSample)
		endpoint := endpointsStore.Get(endpointKey)
		if endpoint == nil {
			// the max number of samples we are going to store and process per endpoint is 10 (it could be configurable)
			endpoint = newEndpoint(10, endpointSample.url)
		}
		if !visitedEndpointsKey.Has(endpointKey) {
			visitedEndpointsKey.Insert(endpointKey)
		}
		endpoint.Add(sample)
		endpointsStore.Add(endpointKeyFunction(endpoint), endpoint)
	}

	hasChanged := false
	for _, visitedEndpointKey := range visitedEndpointsKey.UnsortedList() {
		endpoint := endpointsStore.Get(visitedEndpointKey)
		if fd.policyEvaluatorFn(endpoint) {
			hasChanged = true
			endpointsStore.Add(endpointKeyFunction(endpoint), endpoint)
		}
	}

	fd.store[batchKey] = endpointsStore
	if hasChanged {
		// TODO: propagate if the status changed
	}
}

func (fd *failureDetector) Run(ctx context.Context) {
	// if you ever change the number of workers then you need to provide a thread-safe store
	fd.processor.run(ctx, 1)
}

func (fd *failureDetector) Collector() chan<- *EndpointSample {
	return fd.processor.collectCh
}

func convertToKeySample(epSample *EndpointSample) (string, *Sample) {
	return EndpointSampleKeyFunction(epSample), &Sample{
		err: epSample.err,
	}
}
