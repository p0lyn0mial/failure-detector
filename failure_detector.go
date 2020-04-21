package failure_detector

import (
	"context"
	"time"

	batchqueue "github.com/p0lyn0mial/batch-working-queue"
	ttlstore "github.com/p0lyn0mial/ttl-cache"
	"k8s.io/apimachinery/pkg/util/clock"
)

// failureDetector
// - exposes a channel via Collector() method that allows for collecting samples.
// - samples are batched by the namespace/service and processed by processBatch() method
// - TODO: processBatch() method calls out to external policy function for assessing the endpoints
// - TODO: internal state is replicated to external storage (atomic.Value) for probing via XYZ() method
type failureDetector struct {
	// endpointSampleKeyFn maps collected sample (EndpointSample) for a service to the internal store
	endpointSampleKeyFn KeyFunc

	processor *processor

	// store holds data (in the form of a nested store) per service (namespace/service)
	//
	// it should:
	//  - automatically removed entries (services) that exceed the configured TTL
	//    that would allows us to remove unused/removed services
	//  - be thread-safe
	store Store

	// createStoreFn a helper function for creating a nested store that actually stores endpoints per service
	//
	// it should:
	//  - automatically removed entries (endpoints) that exceed the configured TTL
	//    that would allows us to remove unused/removed endpoints per service
	createStoreFn NewStoreFunc
}

func NewDefaultFailureDetector() *failureDetector {
	createNewStoreFn := func(keyFn KeyFunc, ttl time.Duration) Store {
		return ttlstore.New(keyFn, ttl, clock.RealClock{})
	}
	queue := batchqueue.New(EndpointSampleToServiceKeyFunction)
	return newFailureDetector(EndpointSampleToServiceKeyFunction, createNewStoreFn, queue)
}

func newFailureDetector(endpointSampleKeyKeyFn KeyFunc, createStoreFn NewStoreFunc, queue BatchQueue) *failureDetector {
	fd := &failureDetector{}
	processor := newProcessor(fd.processBatch, queue)
	fd.processor = processor
	fd.store = createStoreFn(wrappedStoreKeyFunction, 120*time.Second)
	fd.endpointSampleKeyFn = endpointSampleKeyKeyFn
	fd.createStoreFn = createStoreFn
	return fd
}

func (fd *failureDetector) processBatch(rawEndpointSample []interface{}) {
	if len(rawEndpointSample) == 0 {
		return
	}
	endpointSamples := convertToEndpointSample(rawEndpointSample)
	batchKey := fd.endpointSampleKeyFn(endpointSamples[0])
	serviceRawStore := fd.store.Get(batchKey)
	if serviceRawStore == nil {
		serviceRawStore = newWrappedStore(fd.createStoreFn(endpointKeyFunction, 60*time.Second), batchKey)
	}
	serviceStore := serviceRawStore.(Store)

	for _, endpointSample := range endpointSamples {
		endpointKey, sample := convertToKeySample(endpointSample)
		rawEndpoint := serviceStore.Get(endpointKey)
		if rawEndpoint == nil {
			// the max number of samples we are going to store and process per endpoint is 10 (it could be configurable)
			rawEndpoint = newEndpoint(10, endpointSample.url)
		}
		endpoint := rawEndpoint.(*Endpoint)
		endpoint.Add(sample)
		serviceStore.Add(endpoint)
	}
	fd.store.Add(serviceStore)
	// TODO: call policy engine
	// TODO: propagate if the status changed
}

func (fd *failureDetector) Run(ctx context.Context, workers int) {
	fd.processor.run(ctx, workers)
}

func (fd *failureDetector) Collector() chan<- *EndpointSample {
	return fd.processor.collectCh
}

func convertToEndpointSample(rawEndpointSample []interface{}) []*EndpointSample {
	ret := make([]*EndpointSample, len(rawEndpointSample))
	for i, r := range rawEndpointSample {
		ret[i] = r.(*EndpointSample)
	}
	return ret
}

func convertToKeySample(epSample *EndpointSample) (string, *Sample) {
	return EndpointSampleKeyFunction(epSample), &Sample{
		err: epSample.err,
	}
}
