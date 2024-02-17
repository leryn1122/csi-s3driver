package kube

import (
	"github.com/leryn1122/csi-s3/pkg/support"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"path/filepath"
)

const kubernetesServiceaccountDirname = "/var/run/secrets/kubernetes.io/serviceaccount"

func CreateKubeClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if IsInKubernetesCluster() {
		config, err = rest.InClusterConfig()
	} else {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func IsInKubernetesCluster() bool {
	exist, err := support.CheckPathExist(kubernetesServiceaccountDirname)
	if err != nil {
		klog.Info("Failed to check path %s exists or not: %s", kubernetesServiceaccountDirname, err.Error())
	}
	return exist
}
