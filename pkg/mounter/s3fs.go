package mounter

import (
	"fmt"
	"github.com/leryn1122/csi-s3/pkg/s3"
	"os"
)

const s3fsCmd = "s3fs"

type s3fsMounter struct {
	metadata      *s3.Metadata
	url           string
	region        string
	pwFileContent string
}

func newS3fsMounter(metadata *s3.Metadata, config *s3.Config) (Mounter, error) {
	return &s3fsMounter{
		metadata:      metadata,
		url:           config.Endpoint,
		region:        config.Region,
		pwFileContent: config.AccessKeyID + ":" + config.SecretAccessKey,
	}, nil
}

func (s3fs *s3fsMounter) Stage(_ string) error {
	return nil
}

//goland:noinspection SpellCheckingInspection
func (s3fs *s3fsMounter) Unstage(_ string) error {
	return nil
}

func (s3fs *s3fsMounter) Mount(_ string, target string) error {
	if err := writeS3fsPassword(s3fs.pwFileContent); err != nil {
		return err
	}
	args := []string{
		fmt.Sprintf("%s:/%s", s3fs.metadata.BucketName, s3fs.metadata.FsPathPrefix),
		target,
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("url=%s", s3fs.url),
		"-o", fmt.Sprintf("endpoint=%s", s3fs.region),
		"-o", "allow_other",
		"-o", "mp_umask=000",
	}
	return fuseMount(target, s3fsCmd, args)
}

func writeS3fsPassword(pwFileContent string) error {
	pwFileName := fmt.Sprintf("%s/.passwd-s3fs", os.Getenv("HOME"))
	pwFile, err := os.OpenFile(pwFileName, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	_, err = pwFile.WriteString(pwFileContent)
	if err != nil {
		return err
	}
	if pwFile.Close() != nil {
		return err
	}
	return nil
}
