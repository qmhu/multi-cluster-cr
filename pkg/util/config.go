package util

import (
	"fmt"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"os"
)

func GetConfigsFromConfigFile(kubeConfigFile string) ([]*rest.Config, error) {
	var configs []*rest.Config
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

		config, err := clientcmd.NewClientConfigFromBytes(apiConfigBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to new client config from bytes: %v", err)
		}

		restConfig, err := config.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to get client config: %v", err)
		}

		configs = append(configs, restConfig)
	}

	return configs, nil
}
