package driver

import (
	"context"
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	mapset "github.com/deckarep/golang-set"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/inhies/go-bytesize"
	"github.com/leryn1122/csi-s3/pkg/constant"
	"github.com/leryn1122/csi-s3/pkg/s3"
	"github.com/mariomac/gostream/stream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	"net/http"
	"strconv"
	"strings"
)

func (d *CSIS3Driver) CreateVolume(ctx context.Context, request *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := d.validateControllerServiceRequestCapability(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		return nil, err
	}

	volumeId := request.GetName()
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name must be provided")
	}
	capabilities := request.GetVolumeCapabilities()
	if capabilities == nil || len(capabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities must be provided")
	}

	secrets := request.GetSecrets()
	mounterType := secrets[constant.TypeKey]
	bucket := secrets[constant.BucketKey]

	if len(bucket) == 0 {
		return nil, status.Error(codes.InvalidArgument, "bucket name count not be empty")
	}

	capacityBytes := request.GetCapacityRange().GetRequiredBytes()
	if capacityBytes == 0 {
		capacityBytes = int64(10 * bytesize.GB)
		klog.Infof("volume capacity has NOT be provided, optional volume capacity is used: %d", capacityBytes)
	}

	pv, err := d.client.CoreV1().PersistentVolumes().Get(ctx, volumeId, metav1.GetOptions{})
	if err != nil && strings.Contains(err.Error(), "not found") {
		switch e := err.(type) {
		case *errors.StatusError:
			if e.Status().Code != int32(http.StatusNotFound) {
				return nil, status.Error(codes.Internal, fmt.Sprintf("failed to fetch PersistVolume %s from API server: %v", volumeId, err.Error()))
			}
		default:
		}
	}
	if pv != nil {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == d.Config.DriverName {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("volume is not created by current CSI: %s", volumeId))
		}
		capacity := pv.Spec.Capacity.Storage().Value()
		if capacity != 0 && capacity != capacityBytes {
			return &csi.CreateVolumeResponse{}, status.Error(codes.AlreadyExists,
				fmt.Sprintf("failed to create a volume with already existing name and different capacity: expected one is %d, and request is %d",
					capacityBytes,
					pv.Spec.Capacity.Storage().Value(),
				))
		}
		klog.Infof("volume already existed: %s", volumeId)
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      volumeId,
				VolumeContext: request.GetParameters(),
				CapacityBytes: capacityBytes,
			},
		}, nil
	}

	klog.Infof("got a request to create volume %s", volumeId)

	metadata := &s3.Metadata{
		BucketName:    bucket,
		Mounter:       mounterType,
		CapacityBytes: capacityBytes,
	}

	// Construct S3 client.
	s3client, err := s3.NewClientFromSecrets(request.GetSecrets())
	s3client.Config.Mounter = mounterType

	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to initialize S3 client: %v", err.Error()))
	}

	// Determine whether the bucket exists.
	// Compare the capacity if exists. Otherwise, create the target bucket.
	exists, err := s3client.BucketExists()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check if bucket `%s` exists: %v", bucket, err.Error()))
	}
	if exists {
		_, err = s3client.GetMetadata()
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to fetch metadata: %v", err.Error()))
		}
	} else {
		if err = s3client.CreateBucket(); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create bucket `%s`: %v", bucket, err.Error()))
		}
	}

	if err = s3client.CreatePrefix(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create prefix: %v", err.Error()))
	}
	if err = s3client.SetMetadata(metadata); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set bucket metadata: %v", err.Error()))
	}

	klog.Infof("Create volume %s", volumeId)
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeId,
			VolumeContext: request.GetParameters(),
			CapacityBytes: capacityBytes,
		},
	}, nil
}

func (d *CSIS3Driver) DeleteVolume(ctx context.Context, request *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if err := d.validateControllerServiceRequestCapability(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		return nil, err
	}

	volumeId := request.GetVolumeId()
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume ID missing in request")
	}

	klog.Infof("got a request to delete volume %s", volumeId)

	pvs, err := d.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", volumeId).String(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get PersistentVolume: %s", err.Error()))
	}
	if len(pvs.Items) > 0 {
		pv := pvs.Items[0]
		if pv.Spec.ClaimRef != nil {
			return nil, status.Error(codes.FailedPrecondition, "volume in use")
		}
	} else {
		return nil, status.Error(codes.Internal, "failed to get PersistentVolume")
	}

	client, err := s3.NewClientFromSecrets(request.GetSecrets())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to initialize S3 client: %s", err.Error()))
	}

	if _, err := client.GetMetadata(); err != nil {
		klog.Infof("FSMeta of volume %s does not exist, ignoring delete request", volumeId)
		return &csi.DeleteVolumeResponse{}, nil
	}

	//var deleteErr error
	//if deleteErr != nil {
	//	klog.Warning("Remove volume failed, will ensure FSMeta exists to avoid losing control over volume")
	//	if err := client.SetMetadata(bucket, metadata); err != nil {
	//		klog.Error(err)
	//	}
	//	return nil, deleteErr
	//}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *CSIS3Driver) ControllerPublishVolume(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume is unimplemented.")
}

func (d *CSIS3Driver) ControllerUnpublishVolume(_ context.Context, _ *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume is unimplemented.")
}

func (d *CSIS3Driver) ValidateVolumeCapabilities(_ context.Context, request *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if len(request.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name missing in request")
	}
	if request.GetVolumeCapabilities() == nil || len(request.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities missing in request")
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: request.GetVolumeCapabilities(),
		},
	}, nil
}

func (d *CSIS3Driver) ListVolumes(ctx context.Context, request *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	startToken := request.StartingToken
	if startToken == "" {
		startToken = "0"
	}
	start, err := strconv.Atoi(startToken)
	if err != nil {
		return &csi.ListVolumesResponse{}, status.Error(codes.Aborted, fmt.Sprintf(
			"the type of starting token should be a integer: %s", request.StartingToken))
	}

	pvs, err := d.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return &csi.ListVolumesResponse{}, status.Error(codes.Internal, fmt.Sprintf("%s", err.Error()))
	}

	entries := stream.Map(
		stream.Of(pvs.Items...).
			Filter(func(pv v1.PersistentVolume) bool {
				if pv.Annotations["pv.kubernetes.io/provisioned-by"] == d.Config.DriverName {
					return true
				}
				return false
			}).
			Filter(func(pv v1.PersistentVolume) bool {
				if pv.Spec.CSI == nil {
					return false
				}
				return pv.Spec.CSI.Driver == d.Config.DriverName
			}).
			Filter(func(pv v1.PersistentVolume) bool {
				return mapset.NewSet(v1.VolumeAvailable, v1.VolumeBound).
					Contains(pv.Status.Phase)
			}).Skip(start),
		func(pv v1.PersistentVolume) *csi.ListVolumesResponse_Entry {
			return &csi.ListVolumesResponse_Entry{
				Volume: &csi.Volume{
					CapacityBytes: pv.Spec.Capacity.Storage().Value(),
					VolumeId:      pv.Name,
					VolumeContext: pv.Spec.CSI.VolumeAttributes,
				},
				Status: &csi.ListVolumesResponse_VolumeStatus{},
			}
		},
	).ToSlice()

	return &csi.ListVolumesResponse{
		Entries:   entries,
		NextToken: strconv.Itoa(len(entries) + 1),
	}, nil
}

func (d *CSIS3Driver) GetCapacity(_ context.Context, _ *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return &csi.GetCapacityResponse{
		//MaximumVolumeSize: &wrappers.Int64Value{Value: stat.Size},
		MinimumVolumeSize: &wrappers.Int64Value{Value: 0},
	}, nil
}

func (d *CSIS3Driver) ControllerGetCapabilities(_ context.Context, _ *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: d.getControllerServiceCapabilities(),
	}, nil
}

func (d *CSIS3Driver) CreateSnapshot(_ context.Context, _ *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot is unsupported")
}

func (d *CSIS3Driver) DeleteSnapshot(_ context.Context, _ *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot is NOT supported.")
}

func (d *CSIS3Driver) ListSnapshots(_ context.Context, _ *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListSnapshots is NOT supported.")
}

func (d *CSIS3Driver) ControllerExpandVolume(_ context.Context, _ *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerExpandVolume is NOT unsupported.")
}

func (d *CSIS3Driver) ControllerGetVolume(_ context.Context, request *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	if err := d.validateControllerServiceRequestCapability(csi.ControllerServiceCapability_RPC_GET_VOLUME); err != nil {
		return nil, err
	}

	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume is NOT supported.")
}

func (d *CSIS3Driver) ControllerModifyVolume(_ context.Context, _ *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerModifyVolume is NOT supported.")
}

func (d *CSIS3Driver) validateControllerServiceRequestCapability(c csi.ControllerServiceCapability_RPC_Type) error {
	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, capability := range d.getControllerServiceCapabilities() {
		if c == capability.GetRpc().GetType() {
			return nil
		}
	}
	return status.Errorf(codes.InvalidArgument, "unsupported capability %s", c)
}

func (d *CSIS3Driver) getControllerServiceCapabilities() []*csi.ControllerServiceCapability {
	return stream.Map(stream.Of(
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_GET_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
	), func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}).ToSlice()
}
