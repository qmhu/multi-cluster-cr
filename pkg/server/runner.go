package server

import (
	"context"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"

	"qmhu/multi-cluster-cr/pkg/config"
	"qmhu/multi-cluster-cr/pkg/known"
)

// managerRunner runs a controller-runtime manager
type managerRunner struct {
	manager   ctrl.Manager
	config    *config.NamedConfig
	closeChan chan struct{}
	closed    chan struct{}
	mu        sync.Mutex
}

func (r *managerRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	runnerCtx, cancel := context.WithCancel(ctx)
	r.closeChan = make(chan struct{})
	r.closed = make(chan struct{})
	r.mu.Unlock()

	// inject cluster name to manager's context
	managerCtx := context.WithValue(runnerCtx, known.ClusterContext, r.config.Name)

	errChan := make(chan error)
	go func() {
		if err := r.manager.Start(managerCtx); err != nil {
			errChan <- err
		}

		close(r.closed)
	}()

	select {
	case err := <-errChan:
		return err
	case <-r.closeChan:
		// call cancel to stop manager
		cancel()

		// wait until manager closed
		<-r.closed
		return nil
	}
}

func (r *managerRunner) Stop() {
	// tell Start() to cancel manager
	close(r.closeChan)

	// wait until manager closed
	<-r.closed
}

