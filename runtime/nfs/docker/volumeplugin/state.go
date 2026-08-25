package volumeplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PastureStack/storage-plugins/runtime/nfs/internal/controlplane"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/sirupsen/logrus"
)

const (
	metadataURL = "http://169.254.169.250/2015-12-19"
	detached    = "detached"
)

var goodStates = map[string]bool{
	"active":            true,
	"activating":        true,
	"deactivating":      true,
	"detached":          true,
	"removing":          true,
	"updating-active":   true,
	"updating-inactive": true,
}

var managedDrivers = map[string]bool{
	"pasturestack-nfs": true,
	"pasturestack-ebs": true,
	"pasturestack-efs": true,
}

type ControlPlaneState struct {
	client   *controlplane.Client
	driver   string
	hostID   string
	driverID string
}

func NewControlPlaneState(driver string, client *controlplane.Client) (*ControlPlaneState, error) {
	host, err := getHostID(client)
	if err != nil {
		return nil, fmt.Errorf("getting host ID: %w", err)
	}
	driverID, err := getDriverID(driver, client)
	if err != nil {
		return nil, fmt.Errorf("getting driver ID: %w", err)
	}

	logrus.Infof("Running on host %s(%s) with driver %s(%s)", host.Hostname, host.Id, driver, driverID)
	return &ControlPlaneState{
		client:   client,
		driver:   driver,
		hostID:   host.Id,
		driverID: driverID,
	}, nil
}

func (r *ControlPlaneState) IsCreated(name string) (bool, error) {
	_, _, err := r.Get(name)
	if err == errNoSuchVolume {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *ControlPlaneState) Save(name string, options map[string]string, try int) error {
	// Wait for the volume resource to be created by the control plane.
	_, vol, err := r.getAny(name)
	for tries := 1; err != nil; tries++ {
		if tries == 30 {
			return fmt.Errorf("maximum retries reached: %w", err)
		}

		_, vol, err = r.getAny(name)

		if err != nil {
			if err == errNoSuchVolume {
				time.Sleep(time.Second)
				continue
			} else {
				return err
			}
		} else {
			break
		}
	}

	opts := &controlplane.Volume{
		Name:            name,
		Driver:          r.driver,
		StorageDriverId: r.driverID,
		DriverOpts:      toMapInterface(options),
		HostId:          r.hostID,
	}

	newVol, err := r.client.Volume.Update(vol, opts)
	if err == nil && (newVol.State == "inactive" || newVol.State == "registering") {
		_, err = r.client.Volume.ActionUpdate(vol)
	}

	logrus.WithField("err", err).Infof("Updating volume %s (%s) %s:%s on %s, state %s => %s",
		opts.Name, vol.Id, opts.Driver, opts.StorageDriverId, opts.HostId,
		vol.State, newVol.State)

	if err != nil {
		if try < 5 {
			try++
			wait := try * 2
			logrus.Warnf("Error while updating volume %s. Sleeping %d and retrying: %v", vol.Id, wait, err)
			time.Sleep(time.Duration(wait) * time.Second)
			return r.Save(name, options, try)
		}
	}

	return err
}

func (r *ControlPlaneState) List() ([]*volume.Volume, error) {
	vols, err := r.client.Volume.List(&controlplane.ListOpts{
		Filters: map[string]interface{}{
			"removed_null":    "true",
			"limit":           "-1",
			"storageDriverId": r.driverID,
		},
	})
	if err != nil {
		return nil, err
	}
	result := []*volume.Volume{}
	for _, vol := range vols.Data {
		if isCreated(r.driver, vol) {
			result = append(result, volToVol(vol))
		}
	}

	return result, nil
}

func isCreated(driver string, vol controlplane.Volume) bool {
	return goodStates[vol.State]
}

func (r *ControlPlaneState) getAny(name string) (*volume.Volume, *controlplane.Volume, error) {
	vols, err := r.client.Volume.List(&controlplane.ListOpts{
		Filters: map[string]interface{}{
			"name":            name,
			"removed_null":    "true",
			"storageDriverId": r.driverID,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	if len(vols.Data) == 0 {
		return nil, nil, errNoSuchVolume
	}

	return volToVol(vols.Data[0]), &vols.Data[0], nil
}

func (r *ControlPlaneState) Get(name string) (*volume.Volume, *controlplane.Volume, error) {
	vols, err := r.client.Volume.List(&controlplane.ListOpts{
		Filters: map[string]interface{}{
			"name":            name,
			"removed_null":    "true",
			"storageDriverId": r.driverID,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	if len(vols.Data) > 1 {
		logrus.Warnf("%d volumes with name=%s found in the control plane", len(vols.Data), name)
	}

	for _, vol := range vols.Data {
		if isCreated(r.driver, vol) {
			return volToVol(vol), &vol, nil
		}
	}

	return nil, nil, errNoSuchVolume
}

func volToVol(vol controlplane.Volume) *volume.Volume {
	result := &volume.Volume{
		Name:   vol.Name,
		Status: map[string]interface{}{},
	}
	bytes, err := json.Marshal(vol)
	if err == nil {
		json.Unmarshal(bytes, &result.Status)
	}
	return result
}

func toMapInterface(data map[string]string) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range data {
		result[k] = v
	}
	return result
}

func getDriverID(driver string, c *controlplane.Client) (string, error) {
	drivers, err := c.StorageDriver.List(&controlplane.ListOpts{
		Filters: map[string]interface{}{
			"name":         driver,
			"removed_null": true,
		},
	})
	if err != nil {
		return "", err
	}
	if len(drivers.Data) != 1 {
		return "", fmt.Errorf("%s is not a driver registered with the current control-plane environment", driver)
	}
	return drivers.Data[0].Id, nil
}

func getHostID(c *controlplane.Client) (*controlplane.Host, error) {
	hostUUID, err := getSelfHostUUID(metadataURL)
	if err != nil {
		return nil, err
	}
	hosts, err := c.Host.List(&controlplane.ListOpts{
		Filters: map[string]interface{}{
			"uuid": hostUUID,
		},
	})
	if err != nil {
		return nil, err
	}
	if len(hosts.Data) != 1 {
		return nil, fmt.Errorf("failed to find current host %s, got %d host(s)", hostUUID, len(hosts.Data))
	}
	return &hosts.Data[0], nil
}

func getSelfHostUUID(baseURL string) (string, error) {
	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/self/host", nil)
		if err == nil {
			request.Header.Set("Accept", "application/json")
			response, requestErr := http.DefaultClient.Do(request)
			if requestErr == nil {
				body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
				response.Body.Close()
				if response.StatusCode == http.StatusOK && readErr == nil {
					var host struct {
						UUID string `json:"uuid"`
					}
					if json.Unmarshal(body, &host) == nil && host.UUID != "" {
						cancel()
						return host.UUID, nil
					}
				}
				lastErr = fmt.Errorf("metadata returned HTTP %d", response.StatusCode)
			} else {
				lastErr = requestErr
			}
		} else {
			lastErr = err
		}
		cancel()
		if time.Now().Add(time.Second).After(deadline) {
			return "", fmt.Errorf("metadata service did not become ready: %w", lastErr)
		}
		time.Sleep(time.Second)
	}
}
