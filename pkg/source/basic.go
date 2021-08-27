package source

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
)

func NewBasicClusterSource(clusters []Cluster) ClusterSource {
	return &BasicClusterSource{
		clusters: clusters,
		channels: make([]chan ClusterEvent, 1),
	}
}

type BasicClusterSource struct {
	clusters []Cluster
	channels []chan ClusterEvent
}

func (s *BasicClusterSource) Start(ctx context.Context) error {
	go func() {
		for _, channel := range s.channels {
			for _, cluster := range s.clusters {
				addEvent := ClusterEvent{
					cluster:   cluster,
					eventType: ClusterEventAdd,
				}

				channel <- addEvent
			}
		}
	}()

	select {
	case <-ctx.Done():
		// exit when recv done
		return nil
	}
}

func (s *BasicClusterSource) Register(eventChan chan ClusterEvent) {
	s.channels = append(s.channels, eventChan)
}

func GenerateClustersFromConfig(kubeConfig string) ([]Cluster, error) {
	var clusters []Cluster
	apiConfig, err := clientcmd.Load([]byte(kubeConfig))
	if err != nil {
		return nil, fmt.Errorf("unable to get api config %v: %v", kubeConfig, err)
	}

	for _, context := range apiConfig.Contexts {
		cluster := Cluster{
			Name: context.Cluster,
		}
		if context.Cluster != apiConfig.CurrentContext {
			clusterApiConfig := apiConfig.DeepCopy()
			clusterApiConfig.CurrentContext = context.Cluster

			apiConfigBytes, err := clientcmd.Write(*clusterApiConfig)
			if err != nil {
				return nil, fmt.Errorf("unable to marshal api config: %v", err)
			}

			cluster.KubeConfig = string(apiConfigBytes)
		} else {
			cluster.KubeConfig = kubeConfig
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}
