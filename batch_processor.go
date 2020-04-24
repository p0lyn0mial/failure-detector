package failure_detector

import (
	"context"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

// processFunc processes a batch of items
type processFunc func(objs []*EndpointSample)

type processor struct {
	batchKey  KeyFunc
	queue     endPointSampleBatchQueue
	processFn processFunc
	collectCh chan *EndpointSample
}

func newProcessor(batchKey KeyFunc, processFn processFunc, queue endPointSampleBatchQueue) *processor {
	return &processor{
		batchKey:  batchKey,
		queue:     queue,
		processFn: processFn,
		collectCh: make(chan *EndpointSample, 1000),
	}
}

func (p *processor) run(ctx context.Context, workers int) {
	// TODO: shutdown the queue
	// defer p.queue.Shutdown()

	for i := 0; i < workers; i++ {
		go wait.Until(p.worker, time.Second, ctx.Done())
	}

	go wait.Until(p.collector(ctx), time.Second, ctx.Done())

	<-ctx.Done()
}

func (p *processor) worker() {
	defer utilruntime.HandleCrash()
	for p.processNextWorkItem() {
	}
}

func (p *processor) processNextWorkItem() bool {
	key, items := p.queue.Get()
	defer p.queue.Done(key)

	// sync
	p.processFn(items)

	return true
}

func (p *processor) collector(ctx context.Context) func() {
	return func() {
		defer utilruntime.HandleCrash()
		for {
			select {
			case <-ctx.Done():
				return
			case item := <-p.collectCh:
				p.queue.Add(p.batchKey(item), item)
			}
		}
	}
}
