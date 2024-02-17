package kube

import (
	"context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCreateKubeClient(t *testing.T) {
	client, err := CreateKubeClient()
	if err != nil {
		panic(err)
	}
	_, err = client.CoreV1().PersistentVolumes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		panic(err)
	}
}
