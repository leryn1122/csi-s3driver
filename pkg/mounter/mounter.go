package mounter

import (
	"errors"
	"fmt"
	"io/ioutil"
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
	TypeKey      = "mounter"
	BucketKey    = "bucket"
	VolumePrefix = "prefix"

	s3fsMounterType   = "s3fs"
	rcloneMounterType = "rclone"
)

func NewMounter(metadata *s3.Metadata, config *s3.Config) (Mounter, error) {
	mounter := metadata.Mounter
	if len(mounter) == 0 {
		mounter = config.Mounter
	}
	switch mounter {
	case s3fsMounterType:
		return newS3fsMounter(metadata, config)
	case rcloneMounterType:
		return newRcloneMounter(metadata, config)
	default:
		return newS3fsMounter(metadata, config)
	}
}

func fuseMounter(path string, command string, args []string) error {
	cmd := exec.Command(command, args...)
	klog.Infof("Mounting fuse with command: %s and args: %s", command, args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error fuseMount command: %s\nargs: %s\noutput", command, args)
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
	return waitForProcess(process)
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
	cmdline, err := ioutil.ReadFile(cmdlineFile)
	if err != nil {
		return "", err
	}
	return string(cmdline), nil
}

func waitForProcess(process *os.Process) error {
	var elapsed time.Duration
	interval := 2 * time.Second
	timeout := 120 * time.Second
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
		if err := process.Signal(syscall.Signal(0)); err != nil {
			klog.Warningf("Fuse process does not seem active or we are unprivileged: %s", err)
			return nil
		}
		klog.Infof("Fuse process with PID %v still active, waiting...", process.Pid)
		time.Sleep(interval)
		elapsed = elapsed + interval
		if elapsed >= timeout {
			return fmt.Errorf("timeout waiting for PID %v to end", process.Pid)
		}
	}
}