package main

import (
	"fmt"
	"io"
	"mydocker/container"
	"os"

	log "github.com/sirupsen/logrus"
)

func logContainer(containerName string) {
	logFileLoc := fmt.Sprintf(container.InfoLocFormat, containerName)
	file, err := os.Open(logFileLoc)
	if err != nil {
		log.Errorf("Log container open file %s error: %v", logFileLoc, err)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Log container read file %s error: %v", logFileLoc, err)
		return
	}
	_, err = fmt.Fprint(os.Stdout, string(content))
	if err != nil {
		log.Errorf("Log container Fprint error: %v", err)
		return
	}
}
