package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

)

const (
	dialTimeout = 5 * time.Second
)

type Watcher struct {
	client  *clientv3.Client
	watches map[string]struct{}
	mu       sync.RWMutex
}

func NewWatcher(client *clientv3.Client) *Watcher {
	return &Watcher{
		client:  client,
		watches: make(map[string]struct{}),
	}
}

func (w *Watcher) Watch(ctx context.Context, key string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.watches[key]; ok {
		return nil
	}

	w.watches[key] = struct{}{}

	go func() {
		ch := w.client.Watch(ctx, key)
		for resp := range ch {
			for _, ev := range resp.Events {
				fmt.Printf("Watch event: %s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		}
	}()

	return nil
}

func (w *Watcher) Unwatch(ctx context.Context, key string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.watches, key)

	return nil
}

func abfs(w *Watcher, key string) {
	visited := make(map[string]bool)
	queue := []string{key}

	for len(queue) > 0 {
		currKey := queue[0]
		queue = queue[1:]

		if visited[currKey] {
			continue
		}

		visited[currKey] = true

		w.Watch(context.Background(), currKey)

		resp, err := w.client.Get(context.Background(), currKey)
		if err != nil {
			log.Println(err)
			continue
		}

		for _, kv := range resp.Kvs {
			queue = append(queue, string(kv.Key))
		}
	}
}

func main() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: dialTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}

	w := NewWatcher(client)

	abfs(w, "/")
}
