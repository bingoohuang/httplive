package httptee

import (
	"context"
	"fmt"
	"log"
)

// Managing CPU load in Golang
// https://blog.ellation.com/managing-cpu-load-in-golang-515b9356bc5

type void = struct{}

// Runnable means anythings that can be run.
type Runnable interface {
	Run() error
}

// Pool means the pool that can be run by pooling workers.
type Pool interface {
	Run(ctx context.Context, job Runnable) error
}

// NewWorkerPool creates a limited pool of permissions in order to limit the number of concurrent jobs.
func NewWorkerPool(maxWorkers int) Pool {
	return &WorkerPool{guard: make(chan void, maxWorkers)}
}

// WorkerPool carries a pool to deliver job.
type WorkerPool struct {
	guard chan void
}

// Run runs the job by the worker.
func (p WorkerPool) Run(ctx context.Context, job Runnable) error {
	answer := make(chan error)
	defer close(answer)

	p.guard <- void{}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println("Recovered in WorkerPool Run from:", r)
				if err, ok := r.(error); ok {
					answer <- err
				} else {
					answer <- fmt.Errorf("Recovered in WorkerPool Run from: %v", r)
				}
			}
			<-p.guard
		}()

		answer <- job.Run()
	}()

	select {
	case err := <-answer:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
