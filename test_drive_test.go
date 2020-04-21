package failure_detector

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"testing"
	"time"
)

func TestDriveFailureDetector(t *testing.T) {
	target := NewDefaultFailureDetector()
	collectorCh := target.Collector()

	go target.Run(context.TODO(), 8)
	for i := 0; i < 100000; i++ {
		collectorCh <- generateRandomItem()
	}
	time.Sleep(10 *time.Second)
}

func generateRandomItem() *EndpointSample {
	return &EndpointSample{
		fmt.Sprintf("%d", rand.Intn(10)),
		"etcd",
		&url.URL{
			Scheme: "https",
			Host:   fmt.Sprintf("1.1.1.%d:6443", rand.Intn(3)),
		},
		nil,
	}
}
