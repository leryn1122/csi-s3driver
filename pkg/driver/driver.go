package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/leryn1122/csi-s3/pkg/kube"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"
)

type CSIS3Driver struct {
	sync.Mutex
	Config Config
	Driver *csicommon.CSIDriver
	client kubernetes.Interface
}

func NewDriver(nodeID string, endpoint string) (*CSIS3Driver, error) {
	config := NewConfig()
	config.NodeID = nodeID
	config.Endpoint = endpoint

	csiDriver := csicommon.NewCSIDriver(config.DriverName, config.Version, nodeID)
	if csiDriver == nil {
		klog.Fatalln("Failed to initialize CSI S3 Driver")
	}

	kubeClient, err := kube.CreateKubeClient()
	if err != nil {
		return nil, err
	}

	driver := &CSIS3Driver{
		Driver: csiDriver,
		Config: config,
		client: kubeClient,
	}
	return driver, nil
}

func (d *CSIS3Driver) Run() error {
	klog.Infof("Driver: %v", d.Config.DriverName)
	klog.Infof("Version: %v", d.Config.Version)

	d.Driver.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME})
	d.Driver.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER})

	grpcServer := csicommon.NewNonBlockingGRPCServer()
	grpc.WithTransportCredentials(insecure.NewCredentials())
	grpcServer.Start(d.Config.Endpoint, d, d, d)
	grpcServer.Wait()

	return nil
}
