package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/leryn1122/csi-s3/pkg/constant"
	"k8s.io/klog/v2"
)

type driver struct {
	driver   *csicommon.CSIDriver
	endpoint string

	ids *identityServer
	cs  *controllerServer
	ns  *nodeServer
}

//goland:noinspection GoExportedFuncWithUnexportedType
func New(nodeID string, endpoint string) (*driver, error) {
	csiDriver := csicommon.NewCSIDriver(constant.DriverName, constant.VendorVersion, nodeID)
	if csiDriver == nil {
		klog.Fatalln("Failed to initialize CSI driver")
	}
	s3Driver := &driver{
		endpoint: endpoint,
		driver:   csiDriver,
	}
	return s3Driver, nil
}

func (s3 *driver) Run() {
	klog.Infof("Driver: %v ", constant.DriverName)
	klog.Infof("Version: %v ", constant.VendorVersion)

	s3.driver.AddControllerServiceCapabilities([]csi.ControllerServiceCapability_RPC_Type{csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME})
	s3.driver.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER})

	s3.ids = s3.newIdentityServer(s3.driver)
	s3.cs = s3.newControllerServer(s3.driver)
	s3.ns = s3.newNodeServer(s3.driver)

	s := csicommon.NewNonBlockingGRPCServer()
	s.Start(s3.endpoint, s3.ids, s3.cs, s3.ns)
	s.Wait()
}

func (s3 *driver) newIdentityServer(d *csicommon.CSIDriver) *identityServer {
	return &identityServer{
		DefaultIdentityServer: csicommon.NewDefaultIdentityServer(d),
	}
}

func (s3 *driver) newControllerServer(d *csicommon.CSIDriver) *controllerServer {
	return &controllerServer{
		DefaultControllerServer: csicommon.NewDefaultControllerServer(d),
	}
}

func (s3 *driver) newNodeServer(d *csicommon.CSIDriver) *nodeServer {
	return &nodeServer{
		DefaultNodeServer: csicommon.NewDefaultNodeServer(d),
	}
}
