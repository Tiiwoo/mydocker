# mydocker

implement a tiny docker

## 准备

准备好 busybox 作为基础镜像，放到 /root/busybox 下

```bash
$ docker pull busybox
$ docker run -d busybox top -d
$ docker export -o busybox.tar [containerID]
$ tar -xvf busybox.tar -C busybox/
```

## Usage

查询 container 运行情况

```bash
mydocker ps
```

运行 container 示例，运行 busybox 镜像，在后台运行 top，并挂在宿主机 /root/from 目录到容器的 /to

```bash
mydocker run -d -name container_name -v /root/from:/to busybox top
```

stop 和 remove 容器

```bash
mydocker stop container_name
mydocker rm container_name
```

容器 commit 到镜像

```bash
mydocker commit container_name image_name
```
