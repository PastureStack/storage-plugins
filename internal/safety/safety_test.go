package safety

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/PastureStack/storage-plugins/internal/model"
)

func TestInputSchemaHasNoDangerousMaterialFields(t *testing.T) {
	inputTypes := []reflect.Type{
		reflect.TypeOf(model.Request{}),
		reflect.TypeOf(model.ObservedState{}),
		reflect.TypeOf(model.Desired{}),
		reflect.TypeOf(model.AliyunBlockConfig{}),
		reflect.TypeOf(model.AWSEBSConfig{}),
		reflect.TypeOf(model.AWSEFSConfig{}),
		reflect.TypeOf(model.NFSConfig{}),
		reflect.TypeOf(model.CephRBDConfig{}),
		reflect.TypeOf(model.LonghornConfig{}),
		reflect.TypeOf(model.LoopConfig{}),
	}
	deniedFragments := []string{"credential", "private", "secret", "socket", "token", "command", "path", "url", "key"}
	for _, inputType := range inputTypes {
		for index := 0; index < inputType.NumField(); index++ {
			field := inputType.Field(index)
			name := strings.ToLower(strings.Split(field.Tag.Get("json"), ",")[0])
			for _, denied := range deniedFragments {
				if denied == "key" && name == "idempotencykey" {
					continue
				}
				if strings.Contains(name, denied) {
					t.Errorf("input type %s exposes denied field %q", inputType.Name(), name)
				}
			}
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
