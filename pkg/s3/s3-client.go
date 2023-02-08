package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"k8s.io/klog/v2"
)

const (
	metadataName = "metadata.json"
	fsPrefix     = "csi-fs"
)

// Config holds values to configure the driver
type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Endpoint        string
	Mounter         string
}

//goland:noinspection GoNameStartsWithPackageName
type S3Client struct {
	Config *Config
	minio  *minio.Client
	ctx    context.Context
}

type Metadata struct {
	BucketName    string `json:"Name"`
	FsPath        string `json:"FsPath"`
	CapacityBytes int64  `json:"CapacityBytes"`
	Mounter       string `json:"Mounter"`
}

func newS3Client(config *Config) (*S3Client, error) {
	u, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, err
	}

	var endpoint string
	if u.Port() != "" {
		endpoint = u.Hostname() + ":" + u.Port()
	} else {
		endpoint = u.Hostname()
	}

	options := &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, config.Region),
		Region: config.Region,
		Secure: u.Scheme == "https",
	}

	var minioClient *minio.Client
	if minioClient, err = minio.New(endpoint, options); err != nil {
		return nil, err
	}
	client := &S3Client{
		Config: config,
		minio:  minioClient,
	}
	return client, err
}

func NewS3ClientFromSecrets(secrets map[string]string) (*S3Client, error) {
	// Mounter is set in the volume preferences, not secrets
	return newS3Client(&Config{
		AccessKeyID:     secrets["accessKeyID"],
		SecretAccessKey: secrets["secretAccessKey"],
		Region:          secrets["region"],
		Endpoint:        secrets["endpoint"],
		Mounter:         "",
	})
}

func (client *S3Client) bucketExists(bucketName string) (bool, error) {
	return client.minio.BucketExists(context.Background(), bucketName)
}

func (client *S3Client) createBucket(bucketName string) error {
	return client.minio.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: client.Config.Region})
}

func (client *S3Client) createPrefix(bucketName string, prefix string) error {
	_, err := client.minio.PutObject(
		context.Background(),
		bucketName,
		prefix+"/",
		bytes.NewReader([]byte("")),
		0,
		minio.PutObjectOptions{
			DisableMultipart: true,
			UserMetadata:     map[string]string{"created by": "csi-s3driver"},
		})
	if err != nil {
		return err
	}
	return nil
}

func (client *S3Client) removeBucket(bucketName string) error {
	if err := client.emptyBucket(bucketName); err != nil {
		return err
	}
	return client.minio.RemoveBucket(context.Background(), bucketName)
}

func (client *S3Client) emptyBucket(bucketName string) error {
	objectsCh := make(chan minio.ObjectInfo)
	var listErr error

	go func() {
		defer close(objectsCh)

		for object := range client.minio.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				listErr = object.Err
				return
			}
			objectsCh <- object
		}
	}()

	if listErr != nil {
		klog.Errorf("Error listing objects: %v", listErr)
		return listErr
	}

	errorCh := client.minio.RemoveObjects(context.Background(), bucketName, objectsCh, minio.RemoveObjectsOptions{})
	for e := range errorCh {
		klog.Errorf("Failed to remove object %q, error: %v", e.ObjectName, e.Err)
	}
	if len(errorCh) != 0 {
		return fmt.Errorf("failed to remove all objects of bucket %s", bucketName)
	}

	// ensure our prefix is also removed
	return client.minio.RemoveObject(context.Background(), bucketName, fsPrefix, minio.RemoveObjectOptions{})
}

func (client *S3Client) metadataExist(bucketName string) bool {
	listOpts := minio.ListObjectsOptions{
		Recursive: false,
		Prefix:    metadataName,
	}
	for objs := range client.minio.ListObjects(context.Background(), bucketName, listOpts) {
		if objs.Err != nil {
			return false
		}
		if objs.ContentType == "application/json" {
			return true
		}
	}
	return false
}

func (client *S3Client) BucketExists(bucketName string) (bool, error) {
	return client.minio.BucketExists(context.Background(), bucketName)
}

func (client *S3Client) CreateBucket(bucketName string) error {
	return client.minio.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{
		Region: client.Config.Region,
	})
}

func (client *S3Client) RemoveBucket(bucketName string) error {
	var err error
	if err = client.removeObjects(bucketName, ""); err == nil {
		return client.minio.RemoveBucket(client.ctx, bucketName)
	}

	klog.Warningf("removeObjects failed with: %s, will try removeObjectsOneByOne", err)

	if err = client.removeObjectsOneByOne(bucketName, ""); err == nil {
		return client.minio.RemoveBucket(client.ctx, bucketName)
	}
	return err
}

func (client *S3Client) SetMetadata(bucketName string, metadata *Metadata) error {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(metadata); err != nil {
		return err
	}
	options := minio.PutObjectOptions{
		ContentType: "application/json",
	}
	_, err := client.minio.PutObject(context.Background(), bucketName, metadataName, b, int64(b.Len()), options)
	return err
}

func (client *S3Client) GetMetadata(bucketName string) (*Metadata, error) {
	opts := minio.GetObjectOptions{}
	obj, err := client.minio.GetObject(context.Background(), bucketName, metadataName, opts)
	if err != nil {
		return nil, err
	}
	objInfo, err := obj.Stat()
	if err != nil {
		return nil, err
	}
	b := make([]byte, objInfo.Size)
	_, err = obj.Read(b)

	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var metadata Metadata
	err = json.Unmarshal(b, &metadata)
	return &metadata, err
}

// CreatePrefix Create a empty "directory".
func (client *S3Client) CreatePrefix(bucketName string, prefix string) error {
	klog.Infof("Prefix: %v", prefix+"/")
	_, err := client.minio.PutObject(context.Background(), bucketName, prefix+"/", bytes.NewReader([]byte("")), 0, minio.PutObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (client *S3Client) RemovePrefix(bucketName string, prefix string) error {
	var err error
	if err = client.removeObjects(bucketName, prefix); err == nil {
		return client.minio.RemoveObject(client.ctx, bucketName, prefix, minio.RemoveObjectOptions{})
	}

	klog.Warningf("removeObjects failed with: %s, will try removeObjectsOneByOne", err)

	if err = client.removeObjectsOneByOne(bucketName, prefix); err == nil {
		return client.minio.RemoveObject(client.ctx, bucketName, prefix, minio.RemoveObjectOptions{})
	}
	return err
}

func (client *S3Client) removeObjects(bucketName string, prefix string) error {
	objectsCh := make(chan minio.ObjectInfo)
	var listErr error

	go func() {
		defer close(objectsCh)

		for object := range client.minio.ListObjects(
			client.ctx,
			bucketName,
			minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err != nil {
				listErr = object.Err
				return
			}
			objectsCh <- object
		}
	}()

	if listErr != nil {
		klog.Error("Error listing objects", listErr)
		return listErr
	}

	select {
	default:
		opts := minio.RemoveObjectsOptions{
			GovernanceBypass: true,
		}
		errorCh := client.minio.RemoveObjects(client.ctx, bucketName, objectsCh, opts)
		haveErrWhenRemoveObjects := false
		for e := range errorCh {
			klog.Errorf("Failed to remove object %s, error: %s", e.ObjectName, e.Err)
			haveErrWhenRemoveObjects = true
		}
		if haveErrWhenRemoveObjects {
			return fmt.Errorf("failed to remove all objects of bucket %s", bucketName)
		}
	}
	return nil
}

func (client *S3Client) removeObjectsOneByOne(bucketName, prefix string) error {
	objectsCh := make(chan minio.ObjectInfo, 1)
	removeErrCh := make(chan minio.RemoveObjectError, 1)
	var listErr error

	go func() {
		defer close(objectsCh)

		for object := range client.minio.ListObjects(client.ctx, bucketName,
			minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err != nil {
				listErr = object.Err
				return
			}
			objectsCh <- object
		}
	}()

	if listErr != nil {
		klog.Error("Error listing objects", listErr)
		return listErr
	}

	go func() {
		defer close(removeErrCh)

		for object := range objectsCh {
			err := client.minio.RemoveObject(client.ctx, bucketName, object.Key,
				minio.RemoveObjectOptions{VersionID: object.VersionID})
			if err != nil {
				removeErrCh <- minio.RemoveObjectError{
					ObjectName: object.Key,
					VersionID:  object.VersionID,
					Err:        err,
				}
			}
		}
	}()

	haveErrWhenRemoveObjects := false
	for e := range removeErrCh {
		klog.Errorf("Failed to remove object %s, error: %s", e.ObjectName, e.Err)
		haveErrWhenRemoveObjects = true
	}
	if haveErrWhenRemoveObjects {
		return fmt.Errorf("failed to remove all objects of path %s", bucketName)
	}

	return nil
}
