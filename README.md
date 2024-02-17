# CSI S3 Driver

The project is still developing, and does NOT work now.

## TL;DR

This is a Container Storage Interface ([CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md)) for S3 (or S3 compatible) storage.

It was inspired from [ctrox/csi-s3](https://github.com/ctrox/csi-s3) repo.

## Table of Contents

- [Background](#background)
- [Install](#install)
- [Related Efforts](#related-efforts)
- [Maintainers](#maintainers)

## Background

- [TencentCloud/kubernetes-csi-tencentcloud](https://github.com/TencentCloud/kubernetes-csi-tencentcloud)
- [kubernetes-sigs/alibaba-cloud-csi-driver](https://github.com/kubernetes-sigs/alibaba-cloud-csi-driver)
- [juicedata/juicefs-csi-driver](https://github.com/juicedata/juicefs-csi-driver)
- [ctrox/csi-s3](https://github.com/ctrox/csi-s3)

## Install

### Prerequisite

 - Kubernetes 1.13+ (CSI v1.0.0 compatibility)
 - Kubernetes has to allow privileged containers
 - Docker daemon must allow shared mounts (systemd flag `MountFlags=shared`)

```bash
# Check systemd flag `MountFlags=shared`
sudo systemctl show --property=MountFlags docker.service

# If the result is not empty, append the flag in section `System` in `docker.service` and restart docker daemon.
[Service]
MountFlags=shared

sudo systemctl daemon-reload
sudo systemctl restart docker.service
```

### Create a secret with S3 credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: csi-s3-secret
  namespace: kube-system
stringData:
  accessKeyID: <YOUR_ACCESS_KEY_ID>
  secretAccessKey: <YOUR_SECRET_ACCES_KEY>
  endpoint: <S3_ENDPOINT_URL>
  region: <S3_REGION>
```

The region could be empty if you are using some other S3 compatible storage.

### Deploy the driver

```bash
cd deploy/kubernetes
find . -name "*.yaml" -exec kubectl apply -f {} \;
```

### Create the storage class

```bash
kubectl create -f examples/storageclass.yaml
```

### 4. Test the S3 driver

1. Create a pvc using the new storage class:

```bash
kubectl create -f examples/pvc.yaml
```

1. Check if the PVC has been bound:

```bash
kubectl get pvc csi-s3driver-pvc

> NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
> csi-s3driver-pvc   Bound    pvc-ff2d5500-0a21-457e-b9d5-06bad56ee369   5Gi        RWO            csi-s3driver   8s
```

Create a test pod which mounts your volume:

```bash
kubectl create -f examples/pod.yaml
```

If the pod can start, everything should be working.

```bash
kubectl exec -ti csi-s3-test-nginx bash
mount | grep fuse

> s3fs on /var/lib/www/html type fuse.s3fs (rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)

touch /var/lib/www/html/hello_world
```

If you stuck when deleting pod or pvc, just run commands below:

```bash
kubectl delete pod csi-s3driver-test-nginx --grace-period=0 --force
kubectl patch pvc pvc-ff2d5500-0a21-457e-b9d5-06bad56ee369 -p '{"metadata":{"finalizers":null}}'
```

## Related Efforts

Those repos are referenced on:

## Maintainers

[@Leryn](https://github.com/leryn1122).