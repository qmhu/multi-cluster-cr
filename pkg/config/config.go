package config

import (
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type NamedConfig struct {
	Name string
	*rest.Config
}

// NewNamedConfig construct a NamedConfig from a name and a rest.Config
func NewNamedConfig(name string, config *rest.Config) *NamedConfig {
	return &NamedConfig{
		Name:   name,
		Config: config,
	}
}

// LoadConfigsFromConfigFile load all clusters and contexts from kubeconfigs and construct to NamedConfig
// use cluster's name to be NamedConfig.Name
func LoadConfigsFromConfigFile(kubeConfigFile string) ([]*NamedConfig, error) {
	var configs []*NamedConfig
	apiConfig, err := clientcmd.LoadFromFile(kubeConfigFile)
	if err != nil {
		return nil, fmt.Errorf("unable to get config from path %v: %v", kubeConfigFile, err)
	}

	for context, _ := range apiConfig.Contexts {
		clusterApiConfig := apiConfig.DeepCopy()
		clusterApiConfig.CurrentContext = context

		apiConfigBytes, err := clientcmd.Write(*clusterApiConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal api config: %v", err)
		}

		config, err := clientcmd.NewClientConfigFromBytes(apiConfigBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to new client config from bytes: %v", err)
		}

		restConfig, err := config.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to get client config: %v", err)
		}

		namedConfig := &NamedConfig{
			Name:   context,
			Config: restConfig,
		}
		configs = append(configs, namedConfig)
	}

	return configs, nil
}
