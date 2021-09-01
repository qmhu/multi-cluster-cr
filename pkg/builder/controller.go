package builder

import (
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Builder struct {
	builder.Builder
}

func (blder *Builder) Build(r reconcile.Reconciler) (controller.Controller, error) {
	// todo: implement
	return nil, nil
}