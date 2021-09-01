package client

import (
	"context"
	"fmt"
)

const ErrClusterContextNotFound = "cluster is not found in context"

func InjectClusterInContext(pc context.Context, cluster string) context.Context {
	return context.WithValue(pc, ClusterContext, cluster)
}

func ExtractClusterFromContext(ctx context.Context) (string, error) {
	val := ctx.Value(ClusterContext)
	if val == nil {
		return "", fmt.Errorf(ErrClusterContextNotFound)
	}

	return val.(string), nil
}