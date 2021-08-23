package source

import "k8s.io/client-go/rest"

type ClusterEventType string

const (
	ClusterEventAdd    ClusterEventType = "Add"
	ClusterEventRemove ClusterEventType = "Remove"
)

type ClusterEvent struct {
	name      string
	eventType ClusterEventType
	kubeconfig string
}

type ClusterSource interface {
	Events() <-chan ClusterEvent
}

func NewClusterSetClusterSource(config *rest.Config, namespace string, name string) ClusterSource {
	return nil
}
