package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
)

type CephDriver struct {
	Mutex       *sync.Mutex
	MountPoint  string
	FSType      string
	Username    string
	Pool        string
	DefaultSize string
}

func New() CephDriver {
	fsType := os.Getenv("CEPH_DRIVER_FSTYPE")
	if fsType == "" {
		fsType = "ext4"
	}
	mountPoint := os.Getenv("CEPH_DRIVER_MOUNTPOINT")
	if mountPoint == "" {
		mountPoint = "/var/lib/ceph-docker-driver"
	}
	userName := os.Getenv("CEPH_DRIVER_USERNAME")
	if userName == "" {
		userName = "admin"
	}
	poolName := os.Getenv("CEPH_DRIVER_POOL")
	if poolName == "" {
		poolName = "rbd"
	}
	defaultSize := os.Getenv("CEPH_DRIVER_DEFAULT_SIZE")
	if defaultSize == "" {
		defaultSize = "8G"
	}
	_, err := os.Lstat(mountPoint)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			fmt.Println("Cannot mkdir")
			os.Exit(1)
		}
	}
	d := CephDriver{
		Mutex:       &sync.Mutex{},
		MountPoint:  mountPoint,
		FSType:      fsType,
		Username:    userName,
		Pool:        poolName,
		DefaultSize: defaultSize,
	}

	return d
}

func (d CephDriver) Capabilities(r volume.Request) volume.Response {
	fmt.Println("Capabilities")
	return volume.Response{Capabilities: volume.Capability{Scope: "global"}}
}

func (d CephDriver) Create(r volume.Request) volume.Response {

	fmt.Println("Create request recieved")
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	size := r.Options["size"]
	if len(size) == 0 {
		size = d.DefaultSize
	}
	err := exec.Command("rbd", "--id", d.Username, "create", "-p", d.Pool, "-s", size, r.Name).Run()
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d CephDriver) Get(r volume.Request) volume.Response {
	fmt.Println("Get")
	err := exec.Command("rbd", "--id", d.Username, "info", "-p", d.Pool, r.Name).Run()
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{Volume: &volume.Volume{Name: r.Name, Mountpoint: d.MountPoint + "/" + r.Name}}
}

func (d CephDriver) Mount(r volume.MountRequest) volume.Response {
	fmt.Println("Staring Mount")
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	watcherCount, err := d.GetWatcherCount(r.Name)
	if watcherCount != 0 {
		return volume.Response{Err: "Volume in use"}
	}
	rbdout, err := exec.Command("rbd", "--id", d.Username, "map", "-p", d.Pool, r.Name).Output()
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	rbd := strings.TrimSuffix(string(rbdout), "\n")
	waitForPathToExist(string(rbd), 60)

	if GetFSType(string(rbd)) == "" {
		fmt.Println("Formatting device")
		err = FormatVolume(string(rbd), d.FSType)
		if err != nil {
			fmt.Println("Failed formatting device")
			return volume.Response{Err: err.Error()}
		}
	}
	_, err = os.Lstat(d.MountPoint + "/" + r.Name)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(d.MountPoint+"/"+r.Name, 0755); err != nil {
			fmt.Println("Cannot mkdir")
			os.Exit(1)
		}
	}
	_, err = exec.Command("mount", string(rbd), d.MountPoint+"/"+r.Name).CombinedOutput()
	if err != nil {
		fmt.Println("Failed to mount:", r.Name)
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{Mountpoint: d.MountPoint + "/" + r.Name, Err: ""}
}

func (d CephDriver) Path(r volume.Request) volume.Response {
	fmt.Println("Starting Path function call")
	_, err := os.Lstat(d.MountPoint + "/" + r.Name)
	if err != nil {
		fmt.Println("Failed to retrieve volume name:", r.Name)
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{Volume: &volume.Volume{Mountpoint: d.MountPoint + "/" + r.Name}}
}

func (d CephDriver) Unmount(r volume.UnmountRequest) volume.Response {
	fmt.Println("Starting Umount")
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	err := exec.Command("rbd", "--id", d.Username, "info", "-p", d.Pool, r.Name).Run()
	if err != nil {
		fmt.Println("Failed to retrieve volume name:", r.Name)
		return volume.Response{Err: err.Error()}
	}
	_, err = os.Lstat(d.MountPoint + "/" + r.Name)
	if os.IsNotExist(err) {
		fmt.Println("No dir to unmount:", r.Name)
	} else {
		_, err = exec.Command("umount", d.MountPoint+"/"+r.Name).CombinedOutput()
		if err != nil {
			if err.Error() == "Volume is not mounted" {
				fmt.Println("Not mounted:", r.Name)
			} else {
				fmt.Println("Failed to umount:", r.Name)
				return volume.Response{Err: err.Error()}
			}
		}
		fmt.Println("Removing Dir")
		_, err = exec.Command("rmdir", d.MountPoint+"/"+r.Name).CombinedOutput()
		if err != nil {
			fmt.Println("Failed to rmdir:", r.Name)
			return volume.Response{Err: err.Error()}
		}
	}

	err = exec.Command("rbd", "--id", d.Username, "unmap", "-p", d.Pool, r.Name).Run()
	if err != nil {
		fmt.Println("Failed to start volume:", r.Name)
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{}
}

func (d CephDriver) Remove(r volume.Request) volume.Response {
	fmt.Println("Remove request recieved")
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	err := exec.Command("rbd", "--id", d.Username, "info", "-p", d.Pool, r.Name).Run()
	if err != nil {
		fmt.Println("Failed to retrieve volume name:", r.Name)
		return volume.Response{Err: err.Error()}
	}
	err = exec.Command("rbd", "--id", d.Username, "rm", "-p", d.Pool, r.Name).Run()
	if err != nil {
		fmt.Println("Failed to retrieve volume name:", r.Name)
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d CephDriver) List(r volume.Request) volume.Response {
	fmt.Println("List request recieved")
	var data []string

	out, err := exec.Command("rbd", "--id", d.Username, "--format", "json", "ls", "-p", d.Pool).Output()
	if err != nil {
		fmt.Println("Failed to run rbd ls command")
		return volume.Response{Err: err.Error()}
	}

	_ = json.Unmarshal(out, &data)

	var vols []*volume.Volume
	for _, d := range data {
		vols = append(vols, &volume.Volume{Name: d})
	}

	return volume.Response{Volumes: vols}
}

func GetFSType(device string) string {
	fmt.Printf("Begin utils.GetFSType: %s", device)
	fsType := ""
	out, err := exec.Command("blkid", device).CombinedOutput()
	if err != nil {
		return fsType
	}

	if strings.Contains(string(out), "TYPE=") {
		for _, v := range strings.Split(string(out), " ") {
			if strings.Contains(v, "TYPE=") {
				fsType = strings.Split(v, "=")[1]
				fsType = strings.Replace(fsType, "\"", "", -1)
			}
		}
	}
	return fsType
}

func FormatVolume(device, fsType string) error {
	fmt.Printf("Begin utils.FormatVolume: %s, %s", device, fsType)
	cmd := "mkfs.ext4"
	if fsType == "xfs" {
		cmd = "mkfs.xfs"
	}
	fmt.Printf("Perform ", cmd, " on device: ", device)
	out, err := exec.Command(cmd, "-F", device).CombinedOutput()
	fmt.Printf("Result of mkfs cmd: ", string(out))
	return err
}

func waitForPathToExist(fileName string, numTries int) bool {
	fmt.Println("Waiting for path")
	for i := 0; i < numTries; i++ {
		_, err := os.Stat(fileName)
		if err == nil {
			fmt.Println("path found: ", fileName)
			return true
		}
		if err != nil && !os.IsNotExist(err) {
			return false
		}
		time.Sleep(time.Second)
		out, err := exec.Command("partprobe").CombinedOutput()
		fmt.Println("Result of partprobe cmd: ", string(out))
	}
	return false
}

type RBDWatcher struct {
	Address string `json:"address"`
	Client  int    `json:"client"`
	Cookie  int    `json:"cookie"`
}

type RBDStatus struct {
	Watchers []RBDWatcher `json:"watchers"`
}

func (d CephDriver) GetWatcherCount(volName string) (int, error) {
	data := RBDStatus{}
	out, _ := exec.Command("rbd", "--id", d.Username, "-p", d.Pool, "--format", "json", "status", volName).Output()
	_ = json.Unmarshal(out, &data)
	return len(data.Watchers), nil
}
