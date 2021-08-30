package source

import (
	"context"
	"fmt"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"os"

	"k8s.io/client-go/tools/clientcmd"
)

func NewBasicClusterSource(clusters []Cluster) ClusterSource {
	return &BasicClusterSource{
		clusters: clusters,
		channels: []chan ClusterEvent{},
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
					Cluster:   cluster,
					EventType: ClusterEventAdd,
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

func GetClustersFromConfigFile(kubeConfigFile string) ([]Cluster, error) {
	var clusters []Cluster
	apiConfig, err := clientcmd.LoadFromFile(kubeConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("unable to get config from path %v: %v", kubeConfigFile, err)
	}

	if apiConfig == nil {
		apiConfig = clientcmdapi.NewConfig()
	}

	for context, _ := range apiConfig.Contexts {
		clusterApiConfig := apiConfig.DeepCopy()
		clusterApiConfig.CurrentContext = context

		apiConfigBytes, err := clientcmd.Write(*clusterApiConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal api config: %v", err)
		}

		cluster := Cluster{
			Name: context,
			KubeConfig: string(apiConfigBytes),
		}

		clusters = append(clusters, cluster)
	}

	return clusters, nil
}
