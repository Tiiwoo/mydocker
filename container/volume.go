package container

import (
	"fmt"
	"mydocker/constant"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

/*
1. 创建 lower 层
2. 创建 upper 和 worker 层
3. 创建 merged 目录并挂载 overlayFS
4. 如果有指定 volume 则挂载 volume
*/
func NewWorkSpace(volume, imageName, containerName string) {
	err := createLower(imageName, containerName)
	if err != nil {
		log.Errorf("createLower err: %v", err)
		return
	}
	err = createUpperWorker(containerName)
	if err != nil {
		log.Errorf("createUpperWorker err: %v", err)
		return
	}
	err = mountOverlayFS(containerName)
	if err != nil {
		log.Errorf("mountOverlayFS err: %v", err)
		return
	}
	if volume != "" {
		volumePaths := volumePathExtract(volume)
		if len(volumePaths) == 2 && volumePaths[0] != "" && volumePaths[1] != "" {
			err = mountVolume(containerName, volumePaths)
			if err != nil {
				log.Errorf("mountVolume err: %v", err)
				return
			}
			// log.Infof("volumePath: %s", volumePaths)
		} else {
			log.Infof("volume parameter input is not correct.")
		}
	}
}

// 容器退出时删除文件系统
/*
1. 有 volume 则卸载 volume
2. 卸载并移除 merged 目录
3. 卸载并移除 upper 和 worker 层
*/
func DeleteWorkSpace(volume, containerName string) error {
	log.Infof("volume: %s, containerName: %s", volume, containerName)
	// 先判断是否有 volume 挂载，如果有则要先 umount volume
	if volume != "" {
		volumePaths := volumePathExtract(volume)
		l := len(volumePaths)
		if l == 2 && volumePaths[0] != "" && volumePaths[1] != "" {
			err := umountVolume(containerName, volumePaths)
			if err != nil {
				return errors.Wrap(err, "umountVolume")
			}
		}
	}
	// 移除相关目录
	err := removeDirs(containerName)
	if err != nil {
		return errors.Wrap(err, "removeDirs")
	}
	// umount 整个容器的挂载点
	err = umountOverlayFS(containerName)
	if err != nil {
		return errors.Wrap(err, "umountOverlayFS")
	}
	// 最后将 /root/containerName 文件夹删除
	root := getRoot(containerName)
	if err = os.RemoveAll(root); err != nil {
		return errors.Wrap(err, "removeRoot")
	}
	return nil
}

// Create Lower
func createLower(imageName, containerName string) error {
	// 得到镜像文件路径和解压路径
	imagePath := getImage(imageName)
	lower := getLower(containerName)

	// 不存在则创建目录并将镜像解压到对应目录
	if err := os.MkdirAll(lower, constant.Perm0622); err != nil {
		return errors.Wrapf(err, "mkdir %s", lower)
	}
	if _, err := exec.Command("tar", "-xvf", imagePath, "-C", lower).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "untar dir %s", lower)
	}
	return nil
}

// 创建 overlayFS 需要的 upper 和 work 目录
func createUpperWorker(containerName string) error {
	upperPath := getUpper(containerName)
	if err := os.MkdirAll(upperPath, constant.Perm0777); err != nil {
		return errors.Wrapf(err, "mkdir dir %s", upperPath)
	}
	workPath := getWorker(containerName)
	if err := os.MkdirAll(workPath, constant.Perm0777); err != nil {
		return errors.Wrapf(err, "mkdir dir %s", workPath)
	}
	return nil
}

// 挂载 overlayfs
func mountOverlayFS(containerName string) error {
	// 创建对应挂载目录
	mntPath := fmt.Sprintf(mergedDirFormat, containerName)
	if err := os.MkdirAll(mntPath, constant.Perm0777); err != nil {
		return errors.Wrapf(err, "mkdir dir %s", mntPath)
	}

	var (
		lower  = getLower(containerName)
		upper  = getUpper(containerName)
		work   = getWorker(containerName)
		merged = getMerged(containerName)
	)
	// lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/merged
	dirs := getOverlayFSDirs(lower, upper, work)
	// mount -t overlay overlay -o lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/work /root/merged
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, merged)
	log.Infof("mountOverlayFS cmd: %s", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return errors.Wrapf(err, "mount dir %s ", mntPath)
}

// 挂载 Volume
func mountVolume(containerName string, volumePaths []string) error {
	// 第 0 个元素为宿主机目录
	parentPath := volumePaths[0]
	// 先判断宿主机目录是否已经创建
	if _, err := os.Stat(parentPath); os.IsNotExist(err) {
		err := os.Mkdir(parentPath, constant.Perm0777)
		if err != nil {
			log.Infof("mkdir parent dir %s error: %v", parentPath, err)
		}
	}
	// 第 1 个元素为容器内部目录
	containerPath := volumePaths[1]
	// 拼接对应容器的目录
	mntPath := getMerged(containerName)
	containerVolumePath := mntPath + "/" + containerPath
	if err := os.Mkdir(containerVolumePath, constant.Perm0777); err != nil {
		log.Infof("mkdir container dir %s error: %v", containerVolumePath, err)
	}
	// 通过 bind mount 将宿主机目录挂载到容器中
	// mount -o bind /hostPath /containerPath
	if _, err := exec.Command("mount", "-o", "bind", parentPath, containerVolumePath).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "bind mount %s to %s", parentPath, containerPath)
	}
	return nil
}

func umountVolume(containerName string, volumePaths []string) error {
	mntPath := getMerged(containerName)
	containerPath := mntPath + "/" + volumePaths[1]
	log.Infof("umount volume path: %s", containerPath)
	if _, err := exec.Command("umount", containerPath).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "umount %s", containerPath)
	}
	return nil
}

func umountOverlayFS(containerName string) error {
	mntPath := getMerged(containerName)
	if _, err := exec.Command("umount", mntPath).CombinedOutput(); err != nil {
		log.Errorf("Umount mountpoint %s failed: %v", mntPath, err)
		return errors.Wrapf(err, "Umount mountpoint %s", mntPath)
	}

	if err := os.RemoveAll(mntPath); err != nil {
		return errors.Wrapf(err, "Remove mountpoint dir %s", mntPath)
	}
	return nil
}

func removeDirs(containerName string) error {
	lower := getLower(containerName)
	upper := getUpper(containerName)
	worker := getWorker(containerName)

	if err := os.RemoveAll(lower); err != nil {
		return errors.Wrapf(err, "remove dir %s", lower)
	}
	if err := os.RemoveAll(upper); err != nil {
		return errors.Wrapf(err, "remove dir %s", upper)
	}
	if err := os.RemoveAll(worker); err != nil {
		return errors.Wrapf(err, "remove dir %s", worker)
	}
	return nil
}

// volumePathExtract 通过冒号分割解析 volume 目录，比如 -v /tmp:/tmp
func volumePathExtract(volume string) []string {
	volumePaths := strings.Split(volume, ":")
	return volumePaths
}

func getRoot(containerName string) string {
	return RootPath + containerName
}

func getImage(imageName string) string {
	return RootPath + imageName + ".tar"
}

func getLower(containerName string) string {
	return fmt.Sprintf(lowerDirFormat, containerName)
}

func getUpper(containerName string) string {
	return fmt.Sprintf(upperDirFormat, containerName)
}

func getWorker(containerName string) string {
	return fmt.Sprintf(workDirFormat, containerName)
}

func getMerged(containerName string) string {
	return fmt.Sprintf(mergedDirFormat, containerName)
}

func getOverlayFSDirs(lower, upper, worker string) string {
	return fmt.Sprintf(overlayFSFormat, lower, upper, worker)
}

func PathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
