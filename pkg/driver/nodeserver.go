package driver

import (
	"context"
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/leryn1122/csi-s3/pkg/constant"
	"github.com/leryn1122/csi-s3/pkg/mounter"
	"github.com/leryn1122/csi-s3/pkg/s3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
	"os"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, request *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	capability := &csi.NodeServiceCapability{
		Type: &csi.NodeServiceCapability_Rpc{
			Rpc: &csi.NodeServiceCapability_RPC{
				Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			},
		},
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			capability,
		},
	}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, request *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	stagingTargetPath := request.GetStagingTargetPath()
	bucketName := request.GetVolumeContext()[constant.BucketKey]
	klog.Infof("Stage volume where VolumeID: %v, Bucket: %v, Stage path: %v", volumeId, bucketName, stagingTargetPath)

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if len(bucketName) == 0 {
		return nil, status.Error(codes.Internal, "Bucket name count not be empty")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if request.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability must be provided")
	}

	err := os.MkdirAll(stagingTargetPath, 0777)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to create mkdir directory for %q error:%v", stagingTargetPath, err))
	}

	s3Client, err := s3.NewS3ClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	metadata, err := s3Client.GetMetadata(bucketName)
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

func (ns *nodeServer) NodePublishVolume(ctx context.Context, request *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	targetPath := request.GetTargetPath()
	stagingTargetPath := request.GetStagingTargetPath()
	bucketName := request.GetVolumeContext()[constant.BucketKey]

	// Validation
	if request.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability must be provided")
	}
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if len(bucketName) == 0 {
		return nil, status.Error(codes.Internal, "Bucket name count not be empty")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target path missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	// Return if already mounted.
	isNotMount, err := checkNotMount(targetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !isNotMount {
		klog.Infof("Target path has been already mounted: %v", targetPath)
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
	s3Client, err := s3.NewS3ClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %s", err)
	}
	metadata, err := s3Client.GetMetadata(bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %s", err)
	}
	mnt, err := mounter.NewMounter(metadata, s3Client.Config)
	if err != nil {
		return nil, err
	}
	if err = mnt.Mount(stagingTargetPath, targetPath); err != nil {
		return nil, err
	}
	klog.Infof("S3 volume `%s` has been successfully mounted to %s", volumeId, targetPath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, request *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	stagingTargetPath := request.GetStagingTargetPath()
	klog.Infof("Unstage volume where VolumeID: %v Stage: %v", volumeId, stagingTargetPath)

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if len(stagingTargetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, request *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	targetPath := request.GetTargetPath()

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	if err := mounter.FuseUmount(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	klog.Infof("S3 volume %s has been unmounted", volumeId)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, request *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}

func checkNotMount(path string) (bool, error) {
	isNotMount, err := mount.New("").IsLikelyNotMountPoint(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0750); err != nil {
				return false, err
			}
		}
		isNotMount = true
	} else {
		return false, err
	}
	return isNotMount, nil
}
