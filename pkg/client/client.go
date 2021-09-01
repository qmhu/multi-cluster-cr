package client

import (
	"context"

	multimanager "qmhu/multi-cluster-cr/pkg/manager"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ErrClusterNotFoundInClientMap = "cluster %s is not found in multi-cluster-map"

type MultiClusterStatusWriter struct {
	MultiClusterClientBase
}

func (mcsw *MultiClusterStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c, err := mcsw.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Status().Update(ctx, obj, opts...)
}

func (mcsw *MultiClusterStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c, err := mcsw.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Status().Patch(ctx, obj, patch, opts...)
}

type MultiClusterClient struct {
	MultiClusterClientBase
	statusWriter client.StatusWriter
}

func NewMultiClusterClientFromManager(mgr multimanager.Manager) client.Client {
	base := &MultiClusterClientBaseImpl{
		mcMap:              mgr.GetClusterMap(),
		defaultClusterName: mgr.GetAdminClusterName(),
	}

	mcc := &MultiClusterClient{
		MultiClusterClientBase: base,
		statusWriter: &MultiClusterStatusWriter{
			MultiClusterClientBase: base,
		},
	}

	return mcc
}

func (mcc *MultiClusterClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Get(ctx, key, obj)
}

func (mcc *MultiClusterClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.List(ctx, list, opts...)
}

func (mcc *MultiClusterClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Create(ctx, obj, opts...)
}

func (mcc *MultiClusterClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Delete(ctx, obj, opts...)
}

func (mcc *MultiClusterClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Update(ctx, obj, opts...)
}

func (mcc *MultiClusterClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.Patch(ctx, obj, patch, opts...)
}

func (mcc *MultiClusterClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	c, err := mcc.getRealClient(ctx)
	if err != nil {
		return err
	}
	return c.DeleteAllOf(ctx, obj, opts...)
}

func (mcc *MultiClusterClient) Status() client.StatusWriter {
	return mcc.statusWriter
}
