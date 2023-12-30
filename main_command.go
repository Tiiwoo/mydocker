package main

import (
	"fmt"

	"mydocker/cgroups/subsystems"
	"mydocker/container"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			mydocker run -it [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it", // 简单起见，这里把 -i 和 -t 参数合并成一个
			Usage: "enable tty",
		},
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
		cli.StringFlag{
			Name:  "cpu",
			Usage: "set cpu quota",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "set cpu share",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "set cpu set",
		},
		cli.StringFlag{
			Name:  "mem",
			Usage: "set memory limit",
		},
		cli.StringFlag{
			Name:  "v",
			Usage: "volume",
		},
	},
	/*
		这里是 run 命令执行的真正函数
		1. 判断参数是否包含 command
		2. 获取用户指定的 command
		3. 调用 Run function 去准备启动容器
	*/
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container command")
		}

		var cmdList []string
		for _, arg := range context.Args() {
			cmdList = append(cmdList, arg)
		}

		// 查找是否有上面定义的 BoolFlag "it"
		tty := context.Bool("it")
		detach := context.Bool("d")

		// tty 和 detach 不能同时提供
		if tty && detach {
			return fmt.Errorf("-it and -d parameter can not use together")
		}
		log.Infof("createTty %v", tty)

		cfg := &subsystems.ResourceConfig{
			CpuCfsQuota: context.Int("cpu"),
			CpuShare:    context.String("cpushare"),
			CpuSet:      context.String("cpuset"),
			MemoryLimit: context.String("mem"),
		}
		// log.Info("Config: ", cfg)
		volume := context.String("v")

		containerName := context.String("name")
		Run(tty, cmdList, cfg, volume, containerName)
		return nil
	},
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in container. Do not call it outside",
	/*
		1. 获取传递过来的 command 参数
		2. 执行容器初始化操作
	*/
	Action: func(context *cli.Context) error {
		log.Infof("init come on")
		err := container.RunContainerInitProcess()
		return err
	},
}

var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit container into image",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container name")
		}
		imgName := context.Args().Get(0)
		commitContainer(imgName)
		return nil
	},
}

var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all the containers",
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print logs of a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("please input your container name")
		}
		containerName := context.Args().Get(0)
		logContainer(containerName)
		return nil
	},
}
