package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/leryn1122/csi-s3/pkg/constant"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"k8s.io/klog/v2"
)

const (
	defaultFSPathPrefix = "csi-fs"
	metadataName        = defaultFSPathPrefix + "/" + "metadata.json"
)

// Config holds values to configure the driver
type Config struct {
	Bucket          string
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
	BucketName    string `json:"driverName"`
	FsPathPrefix  string `json:"fsPathPrefix"`
	CapacityBytes int64  `json:"capacityBytes"`
	Mounter       string `json:"mounter"`
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

func NewClientFromSecrets(secrets map[string]string) (*S3Client, error) {
	// Mounter is set in the volume preferences, not secrets
	return newS3Client(&Config{
		Bucket:          secrets[constant.BucketKey],
		AccessKeyID:     secrets["accessKeyID"],
		SecretAccessKey: secrets["secretAccessKey"],
		Region:          secrets["region"],
		Endpoint:        secrets["endpoint"],
		Mounter:         secrets[constant.TypeKey],
	})
}

func (client *S3Client) bucketExists() (bool, error) {
	return client.minio.BucketExists(context.Background(), client.Config.Bucket)
}

func (client *S3Client) createBucket() error {
	return client.minio.MakeBucket(context.Background(), client.Config.Bucket, minio.MakeBucketOptions{Region: client.Config.Region})
}

func (client *S3Client) createPrefix(prefix string) error {
	_, err := client.minio.PutObject(
		context.Background(),
		client.Config.Bucket,
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

func (client *S3Client) removeBucket() error {
	if err := client.emptyBucket(); err != nil {
		return err
	}
	return client.minio.RemoveBucket(context.Background(), client.Config.Bucket)
}

func (client *S3Client) emptyBucket() error {
	bucket := client.Config.Bucket
	objectsCh := make(chan minio.ObjectInfo)
	var listErr error

	go func() {
		defer close(objectsCh)

		for object := range client.minio.ListObjects(context.Background(), bucket, minio.ListObjectsOptions{Recursive: true}) {
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

	errorCh := client.minio.RemoveObjects(context.Background(), bucket, objectsCh, minio.RemoveObjectsOptions{})
	for e := range errorCh {
		klog.Errorf("Failed to remove object %q, error: %v", e.ObjectName, e.Err)
	}
	if len(errorCh) != 0 {
		return fmt.Errorf("failed to remove all objects of bucket %s", bucket)
	}

	// ensure our prefix is also removed
	return client.minio.RemoveObject(context.Background(), bucket, defaultFSPathPrefix, minio.RemoveObjectOptions{})
}

func (client *S3Client) metadataExist() bool {
	listOpts := minio.ListObjectsOptions{
		Recursive: false,
		Prefix:    metadataName,
	}
	for objs := range client.minio.ListObjects(context.Background(), client.Config.Bucket, listOpts) {
		if objs.Err != nil {
			return false
		}
		if objs.ContentType == "application/json" {
			return true
		}
	}
	return false
}

func (client *S3Client) BucketExists() (bool, error) {
	return client.minio.BucketExists(context.Background(), client.Config.Bucket)
}

func (client *S3Client) CreateBucket() error {
	return client.minio.MakeBucket(context.Background(), client.Config.Bucket, minio.MakeBucketOptions{
		Region: client.Config.Region,
	})
}

func (client *S3Client) StatBucket() (minio.ObjectInfo, error) {
	object, err := client.minio.StatObject(context.Background(), client.Config.Bucket, "", minio.StatObjectOptions{})
	return object, err
}

func (client *S3Client) RemoveBucket() error {
	bucket := client.Config.Bucket

	var err error
	if err = client.removeObjects(""); err == nil {
		return client.minio.RemoveBucket(client.ctx, bucket)
	}

	klog.Warningf("removeObjects failed with: %s, will try removeObjectsOneByOne", err)

	if err = client.removeObjectsOneByOne(""); err == nil {
		return client.minio.RemoveBucket(client.ctx, bucket)
	}
	return err
}

func (client *S3Client) SetMetadata(metadata *Metadata) error {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(metadata); err != nil {
		return err
	}
	options := minio.PutObjectOptions{
		ContentType: "application/json",
	}
	_, err := client.minio.PutObject(context.Background(), client.Config.Bucket, metadataName, b, int64(b.Len()), options)
	return err
}

func (client *S3Client) GetMetadata() (*Metadata, error) {
	opts := minio.GetObjectOptions{}
	obj, err := client.minio.GetObject(context.Background(), client.Config.Bucket, metadataName, opts)
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

// CreatePrefix Create an empty "directory".
func (client *S3Client) CreatePrefix() error {
	klog.Infof("Prefix: %s", defaultFSPathPrefix)
	_, err := client.minio.PutObject(context.Background(), client.Config.Bucket, defaultFSPathPrefix+"/", bytes.NewReader([]byte("")), 0, minio.PutObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (client *S3Client) RemovePrefix(prefix string) error {
	bucket := client.Config.Bucket

	var err error
	if err = client.removeObjects(prefix); err == nil {
		return client.minio.RemoveObject(client.ctx, bucket, prefix, minio.RemoveObjectOptions{})
	}

	klog.Warningf("removeObjects failed with: %s, will try removeObjectsOneByOne", err)

	if err = client.removeObjectsOneByOne(prefix); err == nil {
		return client.minio.RemoveObject(client.ctx, bucket, prefix, minio.RemoveObjectOptions{})
	}
	return err
}

func (client *S3Client) removeObjects(prefix string) error {
	bucket := client.Config.Bucket

	objectsCh := make(chan minio.ObjectInfo)
	var listErr error

	go func() {
		defer close(objectsCh)

		for object := range client.minio.ListObjects(
			client.ctx,
			bucket,
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
		errorCh := client.minio.RemoveObjects(client.ctx, bucket, objectsCh, opts)
		haveErrWhenRemoveObjects := false
		for e := range errorCh {
			klog.Errorf("Failed to remove object %s, error: %s", e.ObjectName, e.Err)
			haveErrWhenRemoveObjects = true
		}
		if haveErrWhenRemoveObjects {
			return fmt.Errorf("failed to remove all objects of bucket %s", bucket)
		}
	}
	return nil
}

func (client *S3Client) removeObjectsOneByOne(prefix string) error {
	bucket := client.Config.Bucket

	objectsCh := make(chan minio.ObjectInfo, 1)
	removeErrCh := make(chan minio.RemoveObjectError, 1)
	var listErr error

	go func() {
		defer close(objectsCh)

		for object := range client.minio.ListObjects(client.ctx, bucket,
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
			err := client.minio.RemoveObject(client.ctx, bucket, object.Key,
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
		return fmt.Errorf("failed to remove all objects of path %s", bucket)
	}

	return nil
}
