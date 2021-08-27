package provider

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	clusternetClientset "github.com/clusternet/clusternet/pkg/generated/clientset/versioned"
	controllerruntimeapi "qmhu/multi-cluster-cr/pkg/apis/controllerruntime/v1alpha1"
	"qmhu/multi-cluster-cr/pkg/source"
)

const (
	ChildClusterSecretName      = "child-cluster-deployer"
	ChildClusterAPIServerURLKey = "apiserver-advertise-url"
)

func NewClusternetProvider(config *rest.Config) (source.ClusterProvider, error) {
	kubeclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to init kubernetes client: %v", err)
	}

	clusternetclient, err := clusternetClientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to init clusternet clientSet: %v", err)
	}

	return &ClusternetProvider{
		clusternetclient: clusternetclient,
		kubeclient:       kubeclient,
	}, nil

}

type ClusternetProvider struct {
	clusternetclient *clusternetClientset.Clientset
	kubeclient       *kubernetes.Clientset
}

func (p *ClusternetProvider) ListClusters(clusterAffinity controllerruntimeapi.ClusterAffinity) ([]source.Cluster, error) {
	if clusterAffinity.Provider != p.ProviderType() {
		return nil, fmt.Errorf("unexpected provider type %s", clusterAffinity.Provider)
	}

	clusterList, err := p.clusternetclient.ClustersV1beta1().ManagedClusters("").List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(clusterAffinity.MatchLabels).String()})
	if err != nil {
		return nil, err
	}

	var clusters []source.Cluster
	for _, clusternetCluster := range clusterList.Items {
		childClusterSecret, err := p.kubeclient.CoreV1().Secrets(clusternetCluster.Namespace).Get(context.TODO(), ChildClusterSecretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		apiConfig := CreateKubeConfigWithToken(
			string(childClusterSecret.Data[ChildClusterAPIServerURLKey]),
			string(childClusterSecret.Data[corev1.ServiceAccountTokenKey]),
			clusternetCluster.Name,
			childClusterSecret.Data[corev1.ServiceAccountRootCAKey],
		)

		kubeconfigBytes, err := clientcmd.Write(*apiConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal api config: %v", err)
		}

		cluster := source.Cluster{
			Name:       clusternetCluster.Name,
			KubeConfig: string(kubeconfigBytes),
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func (p *ClusternetProvider) ProviderType() controllerruntimeapi.Provider {
	return controllerruntimeapi.ClusternetProvider
}

// createBasicKubeConfig creates a basic, general KubeConfig object that then can be extended
func createBasicKubeConfig(serverURL, clusterName, userName string, caCert []byte) *clientcmdapi.Config {
	// Use the cluster and the username as the context name
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	var insecureSkipTLSVerify bool
	if caCert == nil {
		insecureSkipTLSVerify = true
	}

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				InsecureSkipTLSVerify:    insecureSkipTLSVerify,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: contextName,
	}
}

// CreateKubeConfigWithToken creates a KubeConfig object with access to the API server with a token
func CreateKubeConfigWithToken(serverURL, token, clusterName string, caCert []byte) *clientcmdapi.Config {
	userName := "clusternet"
	config := createBasicKubeConfig(serverURL, clusterName, userName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		Token: token,
	}
	return config
}
