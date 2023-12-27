package subsystems

import (
	"fmt"
	"mydocker/constant"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"
)

type MemorySubsystem struct{}

func (s *MemorySubsystem) Name() string {
	return "memory"
}

func (s *MemorySubsystem) Set(cgroupPath string, cfg *ResourceConfig) error {
	if cfg.MemoryLimit == "" {
		return nil
	}
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}

	if err = os.WriteFile(path.Join(subsysCgroupPath, "memory.limit_in_bytes"), []byte(cfg.MemoryLimit), constant.Perm0644); err != nil {
		return fmt.Errorf("set cgroup memory failed %v", err)
	}
	return nil
}

func (s *MemorySubsystem) Apply(cgroupPath string, pid int, cfg *ResourceConfig) error {
	if cfg.MemoryLimit == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return errors.Wrapf(err, "get cgroup %s", cgroupPath)
	}
	if err := os.WriteFile(path.Join(subsysCgroupPath, "tasks"), []byte(strconv.Itoa(pid)), constant.Perm0644); err != nil {
		return fmt.Errorf("add process: %d to cgroup failed %v", pid, err)
	}
	return nil
}

func (s *MemorySubsystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return err
	}
	return os.RemoveAll(subsysCgroupPath)
}
