package volumeplugin

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/PastureStack/storage-plugins/runtime/nfs/internal/controlplane"
	"github.com/sirupsen/logrus"
)

// This key is an inherited wire-protocol marker consumed by the compatible
// control plane. It is not a product or image name.
const legacyManagedMarkerKey = "rancher"

func logRequest(action, name string, options map[string]string) {
	fields := logrus.Fields{}
	if name != "" {
		fields["name"] = name
	}
	if len(options) > 0 {
		keys := make([]string, 0, len(options))
		for key := range options {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fields["optionKeys"] = keys
	}
	logrus.WithFields(fields).Infof("%s.request", action)
}

func logResponse(action, name, mountpoint string, responseErr error, output *CmdOutput) {
	fields := logrus.Fields{}
	fields["name"] = name
	if mountpoint != "" {
		fields["mountpoint"] = mountpoint
	}
	if output.Message != "" {
		fields["message"] = output.Message
	}
	if output.Status != "" {
		fields["status"] = output.Status
	}
	if responseErr != nil {
		fields["error"] = responseErr.Error()
		logrus.WithFields(fields).Errorf("%s.response", action)
	} else {
		logrus.WithFields(fields).Infof("%s.response", action)
	}
}

func getOptions(vol *controlplane.Volume) map[string]string {
	if vol == nil {
		return nil
	}
	result := map[string]string{}
	for k, v := range vol.DriverOpts {
		result[k] = fmt.Sprint(v)
	}
	return result
}

func fold(data ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, d := range data {
		for k, v := range d {
			result[k] = v
		}
	}
	return result
}

func toArgs(name string, data map[string]string) string {
	if data == nil {
		data = map[string]string{}
	}
	data["name"] = name
	data[legacyManagedMarkerKey] = "true"
	bytes, err := json.Marshal(data)
	if err != nil {
		// This really shouldn't ever happen
		panic(err)
	}
	return string(bytes)
}
