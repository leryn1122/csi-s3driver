package mounter

import (
	"github.com/leryn1122/csi-s3/pkg/s3"
)

const rcloneCmd = "rclone"

type rcloneMounter struct {
	metadata      *s3.Metadata
	url           string
	region        string
	pwFileContent string
}

func newRcloneMounter(metadata *s3.Metadata, config *s3.Config) (Mounter, error) {
	return nil, nil
}

func (rclone *rcloneMounter) Stage(stageTarget string) error {
	panic("unimplemented")
}

func (rclone *rcloneMounter) Unstage(stageTarget string) error {
	panic("unimplemented")
}

func (rclone *rcloneMounter) Mount(source string, target string) error {
	panic("unimplemented")
}
