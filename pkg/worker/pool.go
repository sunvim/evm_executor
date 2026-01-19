package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sunvim/evm_executor/pkg/logger"
)

// Task represents a unit of work
type Task interface {
	Execute(ctx context.Context) error
	ID() string
}

// Pool is a worker pool that executes tasks concurrently
type Pool struct {
	name          string
	workerCount   int
	queueSize     int
	taskQueue     chan Task
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	tasksExecuted atomic.Uint64
	tasksFailed   atomic.Uint64
}

// NewPool creates a new worker pool
func NewPool(name string, workerCount, queueSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		name:        name,
		workerCount: workerCount,
		queueSize:   queueSize,
		taskQueue:   make(chan Task, queueSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the worker pool
func (p *Pool) Start() {
	logger.Infof("Starting worker pool %s with %d workers", p.name, p.workerCount)

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// worker is the worker goroutine
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	logger.Debugf("Worker %s-%d started", p.name, id)

	for {
		select {
		case <-p.ctx.Done():
			logger.Debugf("Worker %s-%d stopped", p.name, id)
			return

		case task, ok := <-p.taskQueue:
			if !ok {
				logger.Debugf("Worker %s-%d task queue closed", p.name, id)
				return
			}

			// Execute task
			if err := task.Execute(p.ctx); err != nil {
				logger.Errorf("Worker %s-%d failed to execute task %s: %v",
					p.name, id, task.ID(), err)
				p.tasksFailed.Add(1)
			} else {
				p.tasksExecuted.Add(1)
			}
		}
	}
}

// Submit submits a task to the pool
func (p *Pool) Submit(task Task) error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down")
	case p.taskQueue <- task:
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// SubmitWait submits a task and waits for it to be queued
func (p *Pool) SubmitWait(ctx context.Context, task Task) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down")
	case p.taskQueue <- task:
		return nil
	}
}

// Stop gracefully stops the worker pool
func (p *Pool) Stop() {
	logger.Infof("Stopping worker pool %s", p.name)
	close(p.taskQueue)
	p.wg.Wait()
	logger.Infof("Worker pool %s stopped", p.name)
}

// Shutdown stops the worker pool with context cancellation
func (p *Pool) Shutdown() {
	p.cancel()
	p.Stop()
}

// Stats returns pool statistics
func (p *Pool) Stats() Stats {
	return Stats{
		Name:          p.name,
		WorkerCount:   p.workerCount,
		QueueSize:     p.queueSize,
		QueueLength:   len(p.taskQueue),
		TasksExecuted: p.tasksExecuted.Load(),
		TasksFailed:   p.tasksFailed.Load(),
	}
}

// Stats holds pool statistics
type Stats struct {
	Name          string
	WorkerCount   int
	QueueSize     int
	QueueLength   int
	TasksExecuted uint64
	TasksFailed   uint64
}
