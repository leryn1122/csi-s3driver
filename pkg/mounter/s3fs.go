package mounter

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/leryn1122/csi-s3/pkg/s3"
	"k8s.io/klog/v2"
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
		fmt.Sprintf("%s:/%s", s3fs.metadata.BucketName, s3fs.metadata.FsPath),
		target,
		"-o", "use_path_request_style",
		"-o", fmt.Sprintf("url=%s", s3fs.url),
		"-o", fmt.Sprintf("endpoint=%s", s3fs.region),
		"-o", "allow_other",
		"-o", "mp_umask=000",
	}
	return fuseMount(s3fsCmd, args)
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

func fuseMount(command string, args []string) error {
	cmd := exec.Command(command, args...)
	klog.Infof("Mount fuse with command: %s with args %s", command, args)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Mount fuse with command: %s with args %s\nerror: %s", command, args, string(out))
	}
	return nil
}
