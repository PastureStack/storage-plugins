package volumeplugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PastureStack/storage-plugins/runtime/nfs/internal/controlplane"
	"github.com/docker/go-plugins-helpers/volume"
	mobyevents "github.com/moby/moby/api/types/events"
	dockerClient "github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
	mount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

var errNoSuchVolume = errors.New("no such volume")

const (
	k8sFsType               = "kubernetes.io/fsType"
	fsType                  = "fs-type"
	defaultCompatibilityDir = "/var/lib/pasturestack/volumes"
	DefaultFsType           = "ext4"
	DefaultScope            = "flex"
	state                   = "state"
)

func NewControlPlaneStorageDriver(driver string, client *controlplane.Client, cli *dockerClient.Client) (*ControlPlaneStorageDriver, error) {
	state, err := NewControlPlaneState(driver, client)
	if err != nil {
		return nil, err
	}
	d := &ControlPlaneStorageDriver{
		DriverName:          driver,
		Basedir:             defaultCompatibilityDir,
		Scope:               DefaultScope,
		CreateSupported:     true,
		Command:             driver,
		client:              client,
		state:               state,
		mounter:             mount.NewSafeFormatAndMount(mount.New(""), utilexec.New()),
		FsType:              DefaultFsType,
		cli:                 cli,
		SaveOnAttach:        false,
		mountMap:            map[string]map[string]struct{}{},
		lock:                newKeyedLocker(),
		ManagedControlPlane: managedDrivers[driver],
	}
	if err := d.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}
	go syncMountMap(d, cli)
	d.kickGC()
	go d.watchContainerEvents()
	return d, nil
}

type ControlPlaneStorageDriver struct {
	DriverName          string
	Basedir             string
	Scope               string
	CreateSupported     bool
	Command             string
	client              *controlplane.Client
	state               *ControlPlaneState
	mounter             *mount.SafeFormatAndMount
	FsType              string
	cli                 *dockerClient.Client
	mountLock           sync.Mutex
	SaveOnAttach        bool
	mountMap            map[string]map[string]struct{}
	mountMapLock        sync.RWMutex
	lock                *keyedLocker
	ManagedControlPlane bool
}

func (d *ControlPlaneStorageDriver) init() error {
	_, err := d.exec("init")
	return err
}

func (d *ControlPlaneStorageDriver) Create(request *volume.CreateRequest) (responseErr error) {
	// we need to lock the name to make create idempotency
	if d.ManagedControlPlane {
		d.lock.Lock(request.Name)
		defer d.lock.Unlock(request.Name)
	}

	logRequest("create", request.Name, request.Options)

	output := &CmdOutput{}
	defer func() { logResponse("create", request.Name, "", responseErr, output) }()

	if created, err := d.state.IsCreated(request.Name); err != nil {
		return err
	} else if created {
		return nil
	}

	result := request.Options
	if d.CreateSupported {
		var err error
		*output, err = d.exec("create", toArgs(request.Name, request.Options))
		if err != nil {
			return err
		}
		result = fold(result, output.Options)
	}

	if err := d.state.Save(request.Name, result, 0); err != nil {
		logrus.Errorf("Save volume name=%s failed, err: %s", request.Name, err)
		_, _ = d.exec("delete", toArgs(request.Name, result))
		return err
	}

	return nil
}

func (d *ControlPlaneStorageDriver) List() (*volume.ListResponse, error) {
	volumes, err := d.state.List()
	if err != nil {
		return nil, err
	}
	return &volume.ListResponse{Volumes: volumes}, nil
}

func (d *ControlPlaneStorageDriver) Get(request *volume.GetRequest) (*volume.GetResponse, error) {
	vol, _, err := d.state.Get(request.Name)
	if err != nil {
		return nil, err
	}
	return &volume.GetResponse{Volume: vol}, nil
}

func (d *ControlPlaneStorageDriver) Remove(request *volume.RemoveRequest) (responseErr error) {
	logRequest("remove", request.Name, nil)

	output := &CmdOutput{}
	defer func() { logResponse("remove", request.Name, "", responseErr, output) }()

	_, rVol, err := d.state.Get(request.Name)
	if err == errNoSuchVolume {
		return nil
	} else if err != nil {
		return err
	}

	// Docker removal is deferred until the control plane starts resource removal.
	if rVol.State == "removing" {
		var err error
		*output, err = d.exec("delete", toArgs(request.Name, getOptions(rVol)))
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *ControlPlaneStorageDriver) isMounted(path string) (bool, error) {
	mounts, err := d.mounter.List()
	if err != nil {
		return false, err
	}
	for _, mount := range mounts {
		if mount.Path == path {
			return true, nil
		}
	}

	return false, nil
}

func (d *ControlPlaneStorageDriver) doAttach(name, opts string) (*CmdOutput, error) {
	cmdOutput, err := d.exec("attach", opts)
	if err != nil && err != errNotSupported {
		logrus.Errorf("Failed to attach %s: %v", name, err)
		return nil, err
	}

	return &cmdOutput, nil
}

func (d *ControlPlaneStorageDriver) Attach(request AttachRequest) (response AttachResponse) {
	d.mountLock.Lock()
	defer d.mountLock.Unlock()

	logrus.WithFields(logrus.Fields{
		"name": request.Name,
	}).Info("attach.request")

	output := &CmdOutput{}
	defer func() {
		var err error
		if response.Err != "" {
			err = errors.New(response.Err)
		}
		logResponse("attach", request.Name, "", err, output)
	}()

	_, rVol, err := d.state.Get(request.Name)
	if err != nil {
		response.Err = err.Error()
		return response
	}

	opts := toArgs(request.Name, getOptions(rVol))
	output, err = d.doAttach(request.Name, opts)
	if err != nil {
		response.Err = err.Error()
		return response
	}

	// If SaveOnAttach, update driver Options.
	if d.SaveOnAttach {
		options := getOptions(rVol)
		options["device"] = output.Device
		if err := d.state.Save(request.Name, options, 0); err != nil {
			logrus.Errorf("Save volume name=%s failed, err: %s", request.Name, err)
			response.Err = err.Error()
			return response
		}
	}

	return response
}

func (d *ControlPlaneStorageDriver) Mount(request *volume.MountRequest) (response *volume.MountResponse, responseErr error) {
	d.mountLock.Lock()
	defer d.mountLock.Unlock()

	logrus.WithFields(logrus.Fields{
		"name": request.Name,
	}).Info("mount.request")

	response = &volume.MountResponse{}
	output := &CmdOutput{}
	defer func() {
		mountpoint := ""
		if response != nil {
			mountpoint = response.Mountpoint
		}
		logResponse("mount", request.Name, mountpoint, responseErr, output)
	}()

	_, rVol, err := d.state.Get(request.Name)
	if err != nil {
		return nil, err
	}

	mntDest, err := d.getMntDest(request.Name)
	if err != nil {
		return nil, err
	}
	if mounted, err := d.isMounted(mntDest); err != nil {
		return nil, fmt.Errorf("checking mounts: %w", err)
	} else if mounted {
		logrus.Infof("%s already mounted on %s", request.Name, mntDest)
		response.Mountpoint = mntDest
		return response, nil
	}

	opts := toArgs(request.Name, getOptions(rVol))
	output, err = d.doAttach(request.Name, opts)
	if err != nil && err != errNotSupported {
		logrus.Errorf("Failed to attach %s: %v", request.Name, err)
		return nil, err
	}

	if err := os.MkdirAll(mntDest, 0750); err != nil {
		return nil, err
	}
	*output, err = d.exec("mount", mntDest, output.Device, opts)
	if err != nil {
		logrus.Errorf("Failed to mount %s: %v", request.Name, err)
		return nil, err
	}

	response.Mountpoint = mntDest
	return response, nil
}

func (d *ControlPlaneStorageDriver) getFsType(vol *controlplane.Volume) string {
	fsType, _ := vol.DriverOpts[fsType].(string)
	if fsType == "" {
		fsType, _ = vol.DriverOpts[k8sFsType].(string)
	}
	if fsType == "" {
		fsType = d.FsType
	}
	return fsType
}

func (d *ControlPlaneStorageDriver) Unmount(request *volume.UnmountRequest) (responseErr error) {
	logrus.WithFields(logrus.Fields{
		"name": request.Name,
	}).Info("unmount.request")

	defer func() { logResponse("unmount", request.Name, "", responseErr, &CmdOutput{}) }()

	d.kickGC()
	return nil
}

func (d *ControlPlaneStorageDriver) unmount(mntDest string) error {
	d.mountLock.Lock()
	defer d.mountLock.Unlock()

	logrus.Infof("Unmounting %s", mntDest)
	device, refCount, err := mount.GetDeviceNameFromMount(d.mounter, mntDest)
	if err != nil {
		return fmt.Errorf("find device %s: %w", mntDest, err)
	}

	if _, err := d.exec("unmount", mntDest); err == errNotSupported {
		if err := d.mounter.Unmount(mntDest); err != nil {
			return fmt.Errorf("unmount with mounter %s: %w", mntDest, err)
		}
	} else if err != nil {
		return fmt.Errorf("unmount %s: %w", mntDest, err)
	}

	if refCount != 1 {
		return nil
	}

	logrus.Infof("Detaching %s", device)
	if _, err := d.exec("detach", device); err != nil && err != errNotSupported {
		return fmt.Errorf("detach %s: %w", device, err)
	}

	if _, err := os.Stat(mntDest); err == nil {
		if notmnt, err := d.mounter.IsLikelyNotMountPoint(mntDest); err != nil {
			return fmt.Errorf("look up mount: %w", err)
		} else if notmnt {
			if err := os.Remove(mntDest); err != nil {
				return fmt.Errorf("delete %s: %w", mntDest, err)
			}
		}
	}
	logrus.Infof("Umounting %s done", mntDest)

	return nil
}

func (d *ControlPlaneStorageDriver) Path(request *volume.PathRequest) (*volume.PathResponse, error) {
	mountpoint, err := d.getMntDest(request.Name)
	if err != nil {
		return nil, err
	}
	return &volume.PathResponse{
		Mountpoint: mountpoint,
	}, nil
}

func (d *ControlPlaneStorageDriver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{
		Capabilities: volume.Capability{
			Scope: d.Scope,
		},
	}
}

func (d *ControlPlaneStorageDriver) getMntDest(name string) (string, error) {
	if name == "" || name == "." || name == ".." || len(name) > 255 ||
		strings.ContainsAny(name, "/\\") || strings.IndexFunc(name, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0 {
		return "", errors.New("volume name is not safe for a mount path")
	}
	root := d.getMntRoot()
	destination := filepath.Join(root, name)
	if !pathWithinRoot(root, destination) {
		return "", errors.New("volume mount path escaped the driver root")
	}
	return destination, nil
}

func (d *ControlPlaneStorageDriver) getMntRoot() string {
	return filepath.Join(d.Basedir, d.DriverName)
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (d *ControlPlaneStorageDriver) kickGC() {
	go func() {
		time.Sleep(time.Second)
		if err := d.gc(); err != nil {
			logrus.Errorf("Failed to run GC: %v", err)
		}
	}()
}

func (d *ControlPlaneStorageDriver) gc() error {
	mntRoot := d.getMntRoot()
	mounts, err := d.mounter.List()
	if err != nil {
		return err
	}

	toUnmount := map[string]bool{}
	toCheck := map[string]bool{}
	for _, mount := range mounts {
		if pathWithinRoot(mntRoot, mount.Path) {
			toCheck[mount.Path] = true
		}
	}

	if len(toCheck) == 0 {
		return nil
	}

	d.mountMapLock.RLock()
	for src, ids := range d.mountMap {
		if len(ids) == 0 {
			if toCheck[src] {
				toUnmount[src] = true
			}
		}
	}
	for mount := range toCheck {
		if _, ok := d.mountMap[mount]; !ok {
			logrus.Errorf("Mounted but not registered in Docker: %s", mount)
			toUnmount[mount] = true
		}
	}
	d.mountMapLock.RUnlock()

	var lastErr error
	for mnt := range toUnmount {
		if err := d.unmount(mnt); err != nil {
			lastErr = err
			logrus.Errorf("Failed to unmount %s: %v", mnt, err)
		}
	}

	return lastErr
}

func (d *ControlPlaneStorageDriver) ListAllVolumes() ([]*volume.Volume, error) {
	vols, err := d.state.client.Volume.List(&controlplane.ListOpts{
		Filters: map[string]interface{}{
			"removed_null":    "true",
			"limit":           "-1",
			"storageDriverId": d.state.driverID,
		},
	})
	if err != nil {
		return nil, err
	}
	result := []*volume.Volume{}
	for _, vol := range vols.Data {
		result = append(result, volToVol(vol))
	}
	return result, nil
}

func (d *ControlPlaneStorageDriver) watchContainerEvents() error {
	for {
		ctx, cancel := context.WithCancel(context.Background())
		result := d.cli.Events(ctx, dockerClient.EventsListOptions{})
	stream:
		for {
			select {
			case event, ok := <-result.Messages:
				if !ok {
					break stream
				}
				if event.Action == mobyevents.ActionDestroy {
					logrus.Infof("container %s destroyed", event.Actor.ID)
					d.mountMapLock.Lock()
					for _, mountsMap := range d.mountMap {
						delete(mountsMap, event.Actor.ID)
					}
					d.mountMapLock.Unlock()
					d.kickGC()
				} else if event.Action == mobyevents.ActionStart {
					inspect, err := d.cli.ContainerInspect(context.Background(), event.Actor.ID, dockerClient.ContainerInspectOptions{})
					if err != nil {
						logrus.Errorf("failed to inspect new created container, err: %v", err)
						continue
					}
					d.mountMapLock.Lock()
					for _, mount := range inspect.Container.Mounts {
						if pathWithinRoot(d.getMntRoot(), mount.Source) {
							if ids, ok := d.mountMap[mount.Source]; ok {
								ids[event.Actor.ID] = struct{}{}
							} else {
								d.mountMap[mount.Source] = map[string]struct{}{}
								d.mountMap[mount.Source][event.Actor.ID] = struct{}{}
							}
						}
					}
					d.mountMapLock.Unlock()
				}
			case err, ok := <-result.Err:
				if ok && err != nil && !errors.Is(err, context.Canceled) {
					logrus.Errorf("Docker event stream stopped: %v", err)
				}
				break stream
			}
		}
		cancel()
		time.Sleep(2 * time.Second)
	}
}

func syncMountMap(d *ControlPlaneStorageDriver, cli *dockerClient.Client) {
	for {
		containers, err := cli.ContainerList(context.Background(), dockerClient.ContainerListOptions{})
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		d.mountMapLock.Lock()
		for _, container := range containers.Items {
			for _, mount := range container.Mounts {
				if pathWithinRoot(d.getMntRoot(), mount.Source) {
					if ids, ok := d.mountMap[mount.Source]; ok {
						ids[container.ID] = struct{}{}
					} else {
						d.mountMap[mount.Source] = map[string]struct{}{}
						d.mountMap[mount.Source][container.ID] = struct{}{}
					}
				}
			}
		}
		d.mountMapLock.Unlock()
		time.Sleep(time.Minute * 1)
	}
}
