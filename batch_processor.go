package failure_detector

import (
	"context"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

// processFunc processes a batch of items
type processFunc func(objs []interface{})

type processor struct {
	queue BatchQueue
	processFn processFunc
	collectCh chan *EndpointSample
}

func newProcessor(processFn processFunc, queue BatchQueue) *processor {
	return &processor{
		queue: queue,
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

func (p *processor) sync(items[]interface{}) {
	p.processFn(items)
}

func (p *processor) worker() {
	defer utilruntime.HandleCrash()
	for p.processNextWorkItem() {
	}
}

func (p *processor) processNextWorkItem() bool {
	key, items := p.queue.Get()
	defer p.queue.Done(key)

	p.sync(items)

	return true
}

func (p *processor) collector(ctx context.Context) func(){
	return func() {
		defer utilruntime.HandleCrash()
		for {
			select {
			case <-ctx.Done():
				return
			case item := <-p.collectCh:
				p.queue.Add(item)
			}
		}
	}
}
