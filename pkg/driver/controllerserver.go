package driver

import (
	"context"
	"fmt"
	"github.com/leryn1122/csi-s3/pkg/constant"

	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/leryn1122/csi-s3/pkg/s3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	defaultFSPath = "csi-fs"
)

type controllerServer struct {
	*csicommon.DefaultControllerServer
}

func (cs *controllerServer) ListVolumes(ctx context.Context, request *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, request *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume is not implemented.")
}

func (cs *controllerServer) ControllerGetVolume(ctx context.Context, request *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume is not implemented.")
}

func (cs *controllerServer) CreateVolume(ctx context.Context, request *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	volumeId := request.GetName()
	capacityBytes := request.GetCapacityRange().GetRequiredBytes()
	parameters := request.GetParameters()
	mounterType := parameters[constant.TypeKey]
	bucketName := parameters[constant.BucketKey]
	defaultFSPath := defaultFSPath

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Warningf("Invalid create volume request: %v", request)
		return nil, err
	}

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if len(bucketName) == 0 {
		return nil, status.Error(codes.Internal, "Bucket name count not be empty")
	}
	if request.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}

	klog.Infof("Got a request to create volume %s", volumeId)

	metadata := &s3.Metadata{
		BucketName:    bucketName,
		Mounter:       mounterType,
		CapacityBytes: capacityBytes,
		FsPath:        defaultFSPath,
	}

	// Construct s3 client.
	s3client, err := s3.NewS3ClientFromSecrets(request.GetSecrets())
	s3client.Config.Mounter = mounterType

	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %s", err)
	}

	// Determine whether the bucket exists.
	// Compare the capacity if exists. Otherwise, create the target bucket.
	exists, err := s3client.BucketExists(bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket `%s` exists: %v", bucketName, err)
	}
	if exists {
		meta, err := s3client.GetMetadata(bucketName)
		if err == nil {
			if capacityBytes > meta.CapacityBytes {
				return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("volume with same name: %s but with smaller capacity.", volumeId))
			}
		}
	} else {
		if err = s3client.CreateBucket(bucketName); err != nil {
			return nil, fmt.Errorf("failed to create bucket `%s`: %v", bucketName, err)
		}
	}

	if err := s3client.CreatePrefix(bucketName, defaultFSPath); err != nil {
		return nil, fmt.Errorf("failed to create prefix `%s`: %v", defaultFSPath, err)
		// return nil, fmt.Errorf("failed to create prefix `%s`: %v", path.Join(prefix, defaultFSPath), err)
	}
	if err := s3client.SetMetadata(bucketName, metadata); err != nil {
		return nil, fmt.Errorf("failed to set bucket metadata: %w", err)
	}

	klog.Infof("Create volume %s", volumeId)
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeId,
			CapacityBytes: capacityBytes,
			VolumeContext: request.GetParameters(),
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, request *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeId := request.GetVolumeId()
	bucketName := ""
	var metadata *s3.Metadata

	// Validation
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Infof("Invalid delete volume request: %v", request)
		return nil, err
	}
	klog.Infof("Deleting volume %s", volumeId)

	client, err := s3.NewS3ClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %s", err)
	}

	if metadata, err = client.GetMetadata(bucketName); err != nil {
		klog.Infof("FSMeta of volume %s does not exist, ignoring delete request", volumeId)
		klog.Infof("FSMeta: %v", metadata)
		return &csi.DeleteVolumeResponse{}, nil
	}

	//var deleteErr error
	//if deleteErr != nil {
	//	klog.Warning("Remove volume failed, will ensure FSMeta exists to avoid losing control over volume")
	//	if err := client.SetMetadata(bucketName, metadata); err != nil {
	//		klog.Error(err)
	//	}
	//	return nil, deleteErr
	//}

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, request *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	volumeId := request.GetVolumeId()
	bucketName := request.GetVolumeContext()[constant.BucketKey]

	// Validation
	if len(request.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if len(bucketName) == 0 {
		return nil, status.Error(codes.Internal, "Bucket name count not be empty")
	}
	if request.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}
	client, err := s3.NewS3ClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %s", err)
	}
	exists, err := client.BucketExists(bucketName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("bucket of volume with volumeId %s does not exist", volumeId))
	}
	if _, err := client.GetMetadata(bucketName); err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("metadata of volume with volumeId %s does not exist", volumeId))
	}
	accessMode := &csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	}
	for _, capability := range request.VolumeCapabilities {
		if capability.GetAccessMode().GetMode() != accessMode.GetMode() {
			return &csi.ValidateVolumeCapabilitiesResponse{Message: "Only single node writer is supported"}, nil
		}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: accessMode,
				},
			},
		},
	}, nil
}
