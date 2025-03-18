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

var (
	notificationLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "notification_latency",
		Help:    "Notification latency in milliseconds",
		Buckets: []float64{1, 5, 10, 50, 100, 500},
	})
	notificationThroughput = promauto.NewCounter(prometheus.CounterOpts{
		Name: "notification_throughput",
		Help: "Number of notifications per second",
	})
	memoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "memory_usage",
		Help: "Memory usage in megabytes",
	})
	cpuUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_usage",
		Help: "CPU usage as a percentage",
	})
	averageWatcherNotificationTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "average_watcher_notification_time",
		Help:    "Average time taken to notify watchers in milliseconds",
		Buckets: []float64{1, 5, 10, 50, 100, 500},
	})
	watcherNotificationSuccessRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "watcher_notification_success_rate",
		Help: "Success rate of watcher notifications as a percentage",
	})
	graphTraversalTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "graph_traversal_time",
		Help:    "Time taken to traverse the graph in milliseconds",
		Buckets: []float64{1, 5, 10, 50, 100, 500},
	})
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

							startTime := time.Now()
				notificationLatency.Observe(float64(time.Since(startTime).Milliseconds()))
				notificationThroughput.Inc()
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
func levelizedBFS(w *Watcher, key string) {
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
		startTime := time.Now()
		graphTraversalTime.Observe(float64(time.Since(startTime).Milliseconds()))
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

	levelizedBFS(w, "/")
	go func() {
		for {
			memoryUsage.Set(float64(getMemoryUsage()))
			cpuUsage.Set(float64(getCPUUsage()))
			averageWatcherNotificationTime.Observe(float64(getAverageWatcherNotificationTime()))
			watcherNotificationSuccessRate.Set(float64(getWatcherNotificationSuccessRate()))
			time.Sleep(1 * time.Second)
		}
	}()

	select {}
}
func getMemoryUsage() float64 {
	return 0
}
func getCPUUsage() float64 {
	return 0
}
func getAverageWatcherNotificationTime() float64 {
	return 0
}
func getWatcherNotificationSuccessRate() float64 {
	return 0
}
func getMemoryUsage() float64 {
	vm, err := psutil.VirtualMemory()
	if err != nil {
		return 0
	}
	return float64(vm.UsedPercent)
}
func getCPUUsage() float64 {
	cpu, err := psutil.CPUPercent(0, false)
	if err != nil {
		return 0
	}
	return float64(cpu)
}
