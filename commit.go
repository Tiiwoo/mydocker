package main

import (
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

func commitContainer(imgName string) {
	mountPath := "/root/merged"
	imgTar := "/root/" + imgName + ".tar"
	fmt.Println("commitContainer imageTar:", imgTar)
	if _, err := exec.Command("tar", "-czf", imgTar, "-C", mountPath, ".").CombinedOutput(); err != nil {
		log.Errorf("tar folder %s error %v", mountPath, err)
	}
}
