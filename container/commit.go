package container

import (
	"os/exec"

	"github.com/pkg/errors"
)

func Commit(containerName, imageName string) error {
	mntPath := getMerged(containerName)
	imageTar := getImage(imageName)
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntPath, ".").CombinedOutput(); err != nil {
		return errors.Wrapf(err, "tar folder %s", mntPath)
	}
	return nil
}
