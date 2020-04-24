module github.com/p0lyn0mial/failure-detector

go 1.13

replace github.com/p0lyn0mial/batch-working-queue => ../batch-working-queue

replace github.com/p0lyn0mial/ttl-cache => ../ttl-cache

require (
	github.com/p0lyn0mial/batch-working-queue v0.0.0-00010101000000-000000000000
	github.com/p0lyn0mial/ttl-cache v0.0.0-00010101000000-000000000000
	k8s.io/apimachinery v0.18.0
)
