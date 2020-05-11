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
	rand.Seed(time.Now().UnixNano())

	target := NewDefaultFailureDetector()
	collectorCh := target.Collector()

	go target.Run(context.TODO())
	for i := 0; i < 100000; i++ {
		collectorCh <- generateRandomItem()
	}
	time.Sleep(2 * time.Second)
	generatedSample := generateRandomItem()

	isHealthy, weight := target.EndpointStatus(generatedSample.Namespace, generatedSample.Service, generatedSample.URL)
	fmt.Println(fmt.Sprintf("endpoint (%s/%s/%s) status isHealthy = %v, weight = %v", generatedSample.Namespace, generatedSample.Service, generatedSample.URL.Host, isHealthy, weight))
}

func generateRandomItem() *EndpointSample {
	var err error
	if rand.Intn(2) == 1 {
		err = fmt.Errorf("nasty error")
	}
	return &EndpointSample{
		fmt.Sprintf("%d", rand.Intn(10)),
		"etcd",
		&url.URL{
			Scheme: "https",
			Host:   fmt.Sprintf("1.1.1.%d:6443", rand.Intn(3)),
		},
		err,
	}
}

