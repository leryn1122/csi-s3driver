package driver

import (
	"context"
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/leryn1122/csi-s3/pkg/constant"
	"github.com/leryn1122/csi-s3/pkg/mounter"
	"github.com/leryn1122/csi-s3/pkg/s3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"os"
)

func (d *CSIS3Driver) NodeStageVolume(_ context.Context, request *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	stagingTargetPath := request.GetStagingTargetPath()
	bucket := request.GetSecrets()[constant.BucketKey]
	klog.Infof("Stage volume where VolumeID: %s, Bucket: %s, Stage path: %s", volumeId, bucket, stagingTargetPath)

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name missing in request")
	}
	if len(bucket) == 0 {
		return nil, status.Error(codes.InvalidArgument, "bucket name count not be empty")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "staging target path missing in request")
	}
	if request.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability must be provided")
	}

	err := os.MkdirAll(stagingTargetPath, 0777)
	if err != nil {
		if !os.IsExist(err) {
			return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to create mkdir directory for %s error:%s", stagingTargetPath, err))
		}
	}

	s3Client, err := s3.NewClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	metadata, err := s3Client.GetMetadata()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get metadata: %v", err))
	}

	mnt, err := mounter.NewMounter(metadata, s3Client.Config)
	if mnt == nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Failed to initialize the moutner: %v", metadata.Mounter))
	}
	if err := mnt.Stage(stagingTargetPath); err != nil {
		return nil, err
	}
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *CSIS3Driver) NodeUnstageVolume(_ context.Context, request *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	stagingTargetPath := request.GetStagingTargetPath()

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name missing in request")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "staging target path missing in request")
	}

	klog.Infof("unstage volume where volumeId: %s stage: %s", volumeId, stagingTargetPath)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *CSIS3Driver) NodePublishVolume(_ context.Context, request *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	targetPath := request.GetTargetPath()
	stagingTargetPath := request.GetStagingTargetPath()
	bucketName := request.GetSecrets()[constant.BucketKey]

	// Validation
	if request.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability must be provided")
	}
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name missing in request")
	}
	if len(bucketName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "bucket name count not be empty")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "staging target path missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "target path missing in request")
	}

	// Check mount point
	isMount, err := mounter.CheckMount(volumeId, targetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check mounted path: %s", err.Error()))
	}
	if isMount {
		klog.Infof("target path has been already mounted: %v", targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	deviceId := ""
	if request.GetPublishContext() != nil {
		deviceId = request.GetPublishContext()[deviceId]
	}

	readonly := request.GetReadonly()
	attributes := request.GetVolumeContext()
	mountFlags := request.GetVolumeCapability().GetMount().GetMountFlags()

	klog.Infof("target = %v\ndevice = %v\nreadonly = %v\nvolumeId = %v\nattributes = %v\nmountFlags = %v\n",
		targetPath, deviceId, readonly, volumeId, attributes, mountFlags)

	// Mount target path by given `mounter`
	s3Client, err := s3.NewClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to initialize S3 client: %s", err.Error()))
	}

	metadata, err := s3Client.GetMetadata()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get metadata: %s", err.Error()))
	}

	mnt, err := mounter.NewMounter(metadata, s3Client.Config)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create mounter: %s", err.Error()))
	}

	if err = mnt.Mount(stagingTargetPath, targetPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mounte path: %s", err.Error()))
	}
	klog.Infof("S3 volume `%s` has been successfully mounted to %s", volumeId, targetPath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *CSIS3Driver) NodeUnpublishVolume(_ context.Context, request *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	targetPath := request.GetTargetPath()

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "target path missing in request")
	}

	if err := mounter.FuseUmount(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("S3 volume %s has been unmounted from %s", volumeId, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *CSIS3Driver) NodeGetVolumeStats(_ context.Context, request *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	stagingTargetPath := request.GetStagingTargetPath()
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "staging target path missing in request")
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Total:     0,
				Available: 0,
				Used:      0,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Total:     0,
				Available: 0,
				Used:      0,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
		VolumeCondition: &csi.VolumeCondition{},
	}, nil
}

func (d *CSIS3Driver) NodeExpandVolume(_ context.Context, _ *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeExpandVolume is unsupported")
}

func (d *CSIS3Driver) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: d.getNodeServiceCapabilities(),
	}, nil
}

func (d *CSIS3Driver) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.Config.NodeID,
	}, nil
}

func (d *CSIS3Driver) getNodeServiceCapabilities() []*csi.NodeServiceCapability {
	cl := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
	}

	var capabilities []*csi.NodeServiceCapability
	for _, cap := range cl {
		capabilities = append(capabilities, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}
	return capabilities
}

func (d *CSIS3Driver) validateNodeServiceCapabilities(c csi.NodeServiceCapability_RPC_Type) error {
	if c == csi.NodeServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, capability := range d.getNodeServiceCapabilities() {
		if c == capability.GetRpc().GetType() {
			return nil
		}
	}
	return status.Errorf(codes.InvalidArgument, "unsupported capability %s", c)
}
