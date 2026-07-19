package contract

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/PastureStack/storage-plugins/internal/model"
)

const validAliyunCreate = `{"apiVersion":"storage-plugins.pasturestack.io/v1alpha1","kind":"LifecyclePlanRequest","driver":"aliyun-block","operation":"create","idempotencyKey":"request-001","ownerId":"team-a","expectedGeneration":0,"observedState":{"phase":"absent","generation":0,"ownerId":""},"desired":{"phase":"available","allowIrreversible":false,"config":{"diskCategory":"cloud-ssd","sizeGiB":20,"snapshotRef":"ref:snapshot-001"}}}`

func TestDecodeAcceptsStrictRequest(t *testing.T) {
	request, err := Decode(strings.NewReader(validAliyunCreate))
	if err != nil {
		t.Fatal(err)
	}
	if request.Driver != model.DriverAliyunBlock || request.Operation != model.OperationCreate {
		t.Fatalf("unexpected request: %#v", request)
	}
}

func TestDecodeRejectsMalformedOrUnsafeStructure(t *testing.T) {
	deepValue := strings.Repeat("[", model.MaxJSONDepth+1) + "0" + strings.Repeat("]", model.MaxJSONDepth+1)
	tests := []struct {
		name string
		data []byte
		code string
	}{
		{name: "exact case root", data: []byte(strings.Replace(validAliyunCreate, `"driver"`, `"Driver"`, 1)), code: ErrorInvalidRequest},
		{name: "exact case nested", data: []byte(strings.Replace(validAliyunCreate, `"diskCategory"`, `"DiskCategory"`, 1)), code: ErrorInvalidRequest},
		{name: "duplicate root key", data: []byte(strings.Replace(validAliyunCreate, `"driver":"aliyun-block",`, `"driver":"aliyun-block","driver":"aliyun-block",`, 1)), code: ErrorInvalidJSON},
		{name: "unknown config field", data: []byte(strings.Replace(validAliyunCreate, `"sizeGiB":20`, `"sizeGiB":20,"command":"blocked"`, 1)), code: ErrorInvalidRequest},
		{name: "secret field", data: []byte(strings.Replace(validAliyunCreate, `"sizeGiB":20`, `"sizeGiB":20,"token":"do-not-accept"`, 1)), code: ErrorInvalidRequest},
		{name: "url field", data: []byte(strings.Replace(validAliyunCreate, `"sizeGiB":20`, `"sizeGiB":20,"url":"https://invalid.example"`, 1)), code: ErrorInvalidRequest},
		{name: "multiple documents", data: []byte(validAliyunCreate + ` {}`), code: ErrorInvalidJSON},
		{name: "escaped control character", data: []byte(strings.Replace(validAliyunCreate, `"ownerId":"team-a"`, `"ownerId":"team\u0000a"`, 1)), code: ErrorInvalidJSON},
		{name: "depth limit", data: []byte(strings.Replace(validAliyunCreate, `"sizeGiB":20`, `"sizeGiB":20,"extra":`+deepValue, 1)), code: ErrorInvalidJSON},
		{name: "invalid utf8", data: append([]byte(validAliyunCreate[:len(validAliyunCreate)-1]), 0xff, '}'), code: ErrorInvalidJSON},
		{name: "too large", data: bytes.Repeat([]byte{' '}, model.MaxInputSize+1), code: ErrorInputTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(bytes.NewReader(test.data))
			if err == nil {
				t.Fatal("Decode unexpectedly succeeded")
			}
			var inputError *InputError
			if !errors.As(err, &inputError) {
				t.Fatalf("got error %T, want *InputError", err)
			}
			if inputError.Code != test.code {
				t.Fatalf("got code %q, want %q", inputError.Code, test.code)
			}
		})
	}
}

func TestDecodeRejectsNullAtEveryDepthAndType(t *testing.T) {
	awsCreate := requestJSON("aws-ebs", `{"volumeType":"gp3","sizeGiB":100,"iops":3000,"encrypted":true,"snapshotRef":"ref:snapshot-001","kmsRef":"ref:kms-policy-a"}`)
	nfsCreate := requestJSON("nfs", `{"serverRef":"ref:nfs-server","exportRef":"ref:team-export","mountOptions":["hard","nfs-v4.1"],"purgePolicy":"retain"}`)

	tests := []struct {
		name  string
		input string
	}{
		{name: "root", input: `null`},
		{name: "root string", input: strings.Replace(validAliyunCreate, `"driver":"aliyun-block"`, `"driver":null`, 1)},
		{name: "nested string", input: strings.Replace(validAliyunCreate, `"diskCategory":"cloud-ssd"`, `"diskCategory":null`, 1)},
		{name: "root uint", input: strings.Replace(validAliyunCreate, `"expectedGeneration":0`, `"expectedGeneration":null`, 1)},
		{name: "nested uint", input: strings.Replace(validAliyunCreate, `"sizeGiB":20`, `"sizeGiB":null`, 1)},
		{name: "nested bool", input: strings.Replace(validAliyunCreate, `"allowIrreversible":false`, `"allowIrreversible":null`, 1)},
		{name: "driver bool", input: strings.Replace(awsCreate, `"encrypted":true`, `"encrypted":null`, 1)},
		{name: "observed object", input: strings.Replace(validAliyunCreate, `"observedState":{"phase":"absent","generation":0,"ownerId":""}`, `"observedState":null`, 1)},
		{name: "desired object", input: strings.Replace(validAliyunCreate, `"desired":{"phase":"available","allowIrreversible":false,"config":{"diskCategory":"cloud-ssd","sizeGiB":20,"snapshotRef":"ref:snapshot-001"}}`, `"desired":null`, 1)},
		{name: "config object", input: strings.Replace(validAliyunCreate, `"config":{"diskCategory":"cloud-ssd","sizeGiB":20,"snapshotRef":"ref:snapshot-001"}`, `"config":null`, 1)},
		{name: "optional array", input: strings.Replace(nfsCreate, `"mountOptions":["hard","nfs-v4.1"]`, `"mountOptions":null`, 1)},
		{name: "array element", input: strings.Replace(nfsCreate, `"mountOptions":["hard","nfs-v4.1"]`, `"mountOptions":["hard",null]`, 1)},
		{name: "map value", input: strings.Replace(validAliyunCreate, `"sizeGiB":20`, `"sizeGiB":20,"metadata":{"value":null}`, 1)},
		{name: "optional resource reference", input: strings.Replace(validAliyunCreate, `"ownerId":""}`, `"ownerId":"","resourceRef":null}`, 1)},
		{name: "optional aliyun snapshot reference", input: strings.Replace(validAliyunCreate, `"snapshotRef":"ref:snapshot-001"`, `"snapshotRef":null`, 1)},
		{name: "optional aws snapshot reference", input: strings.Replace(awsCreate, `"snapshotRef":"ref:snapshot-001"`, `"snapshotRef":null`, 1)},
		{name: "optional aws kms reference", input: strings.Replace(awsCreate, `"kmsRef":"ref:kms-policy-a"`, `"kmsRef":null`, 1)},
	}

	for _, driver := range []string{"aliyun-block", "aws-ebs", "aws-efs", "ceph-rbd", "longhorn", "loop", "nfs"} {
		tests = append(tests, struct {
			name  string
			input string
		}{name: "config for " + driver, input: requestJSON(driver, `null`)})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(test.input))
			if err == nil {
				t.Fatal("Decode unexpectedly accepted null")
			}
			var inputError *InputError
			if !errors.As(err, &inputError) {
				t.Fatalf("got error %T, want *InputError", err)
			}
			if inputError.Code != ErrorInvalidJSON {
				t.Fatalf("got code %q, want %q", inputError.Code, ErrorInvalidJSON)
			}
		})
	}
}

func TestDecodeNormalizesAllowlistedMountOptions(t *testing.T) {
	input := requestJSON("nfs", `{"serverRef":"ref:nfs-server","exportRef":"ref:team-export","mountOptions":["sync","hard","nfs-v4.1"],"purgePolicy":"retain"}`)
	request, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	config, ok := request.Desired.Config.(model.NFSConfig)
	if !ok {
		t.Fatalf("got config %T", request.Desired.Config)
	}
	want := []string{"hard", "nfs-v4.1", "sync"}
	if strings.Join(config.MountOptions, ",") != strings.Join(want, ",") {
		t.Fatalf("got options %v, want %v", config.MountOptions, want)
	}
}

func requestJSON(driver, config string) string {
	return `{"apiVersion":"storage-plugins.pasturestack.io/v1alpha1","kind":"LifecyclePlanRequest","driver":"` + driver + `","operation":"create","idempotencyKey":"request-001","ownerId":"team-a","expectedGeneration":0,"observedState":{"phase":"absent","generation":0,"ownerId":""},"desired":{"phase":"available","allowIrreversible":false,"config":` + config + `}}`
}
