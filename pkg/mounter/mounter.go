package mounter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/leryn1122/csi-s3/pkg/s3"
	"github.com/mitchellh/go-ps"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

type Mounter interface {
	Stage(stagePath string) error
	Unstage(stagePath string) error
	Mount(source string, target string) error
}

const (
	S3fsMounterType   = "s3fs"
	RcloneMounterType = "rclone"
	GoofysMounterType = "goofys"
)

func NewMounter(metadata *s3.Metadata, config *s3.Config) (Mounter, error) {
	mounter := metadata.Mounter
	if len(mounter) == 0 {
		mounter = config.Mounter
	}
	switch mounter {
	case S3fsMounterType:
		return newS3fsMounter(metadata, config)
	case RcloneMounterType:
		return newRcloneMounter(metadata, config)
	case GoofysMounterType:
		return newGoofysMounter(metadata, config)
	default:
		klog.Errorf("unknown mounter %s, using default mounter %s", mounter, S3fsMounterType)
		return newS3fsMounter(metadata, config)
	}
}

func fuseMount(path string, command string, args []string) error {
	cmd := exec.Command(command, args...)
	klog.Infof("Mount fuse with command: %s with args %s", command, args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Mount fuse mount with command: %s with args %s\nerror: %s", command, args, string(out))
	}
	return waitForMount(path, 10*time.Second)
}

func waitForMount(path string, timeout time.Duration) error {
	var elapsed time.Duration
	var interval = 10 * time.Millisecond
	for {
		notMount, err := mount.New("").IsLikelyNotMountPoint(path)
		if err != nil {
			return err
		}
		if !notMount {
			return nil
		}
		time.Sleep(interval)
		elapsed = elapsed + interval
		if elapsed >= timeout {
			return errors.New("timeout waiting for mount")
		}
	}
}

func FuseUmount(path string) error {
	if err := mount.New("").Unmount(path); err != nil {
		return err
	}
	// Wait until the process is done, while FUSE quits immediately.
	process, err := findFuseMountProcess(path)
	if err != nil {
		klog.Errorf("Failed to obtain PID of fuse mount %s: %s", path, err)
	}
	if process == nil {
		klog.Warningf("Unable to find PID of fuse mount %s, it must be already finished", path)
		return nil
	}
	klog.Infof("Found fuse PID %v of fuse mount %s, checking if it still runs", process.Pid, path)
	return waitForProcess(process, 0)
}

func findFuseMountProcess(path string) (*os.Process, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	for _, process := range processes {
		cmdline, err := getCmdline(process.Pid())
		if err != nil {
			klog.Errorf("Unable to get cmdline of PID %v: %s", process.Pid(), err)
			continue
		}
		if strings.Contains(cmdline, path) {
			klog.Errorf("Find process %v mounting on path %s", process.Pid(), path)
			return os.FindProcess(process.Pid())
		}
	}
	return nil, nil
}

func getCmdline(pid int) (string, error) {
	cmdlineFile := fmt.Sprintf("/proc/%v/cmdline", pid)
	cmdline, err := os.ReadFile(cmdlineFile)
	if err != nil {
		return "", err
	}
	return string(cmdline), nil
}

func waitForProcess(process *os.Process, backoff int) error {
	if backoff == 20 {
		return fmt.Errorf("time waiting for PID %v to end", process.Pid)
	}

	interval := 2 * time.Second
	for {
		cmdline, err := getCmdline(process.Pid)
		if err != nil {
			klog.Warningf("Fail to check cmdline of PID %v, assuming it is dead: %s", process.Pid, err)
			return nil
		}
		if cmdline == "" {
			klog.Warning("Fuse process seems dead, killed")
			proc, _ := os.FindProcess(process.Pid)
			return proc.Kill()
		}
		if err = process.Signal(syscall.Signal(0)); err != nil {
			klog.Warningf("Fuse process does not seem active or we are unprivileged: %s", err)
			return nil
		}
		klog.Infof("Fuse process with PID %v still active, waiting...", process.Pid)
		time.Sleep(interval + time.Duration(backoff*100))
		return waitForProcess(process, backoff+1)
	}
}

func CheckMount(volumeId, path string) (bool, error) {
	isMount, err := mount.New("").IsLikelyNotMountPoint(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0750); err != nil {
				return false, err
			}
		}
		isMount = false
	} else {
		return false, err
	}
	return isMount, nil
}
