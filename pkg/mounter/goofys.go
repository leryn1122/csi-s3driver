package mounter

import "github.com/leryn1122/csi-s3/pkg/s3"

const goofysCmd = "goofys"

type goofysMounter struct {
}

func newGoofysMounter(metadata *s3.Metadata, config *s3.Config) (Mounter, error) {
	return &goofysMounter{}, nil
}

func (goofys *goofysMounter) Stage(stageTarget string) error {
	panic("unimplemented")
}

func (goofys *goofysMounter) Unstage(stageTarget string) error {
	panic("unimplemented")
}

func (goofys *goofysMounter) Mount(source string, target string) error {
	panic("unimplemented")
}
