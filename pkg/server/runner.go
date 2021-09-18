package server

import (
	"context"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"

	"qmhu/multi-cluster-cr/pkg/config"
	"qmhu/multi-cluster-cr/pkg/known"
)

type managerRunner struct {
	manager      ctrl.Manager
	config       *config.NamedConfig
	cancel       context.CancelFunc
	stopComplete chan struct{}
	mu           sync.Mutex
}

func (r *managerRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	runnerCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.stopComplete = make(chan struct{})
	r.mu.Unlock()

	managerCtx := context.WithValue(runnerCtx, known.ClusterContext, r.config.Name)

	return r.manager.Start(managerCtx)
}

func (r *managerRunner) Stop() {
	r.cancel()

	select {
	case <-r.stopComplete:
		return
	}
}

func (r *managerRunner) Done() {
	close(r.stopComplete)
}
