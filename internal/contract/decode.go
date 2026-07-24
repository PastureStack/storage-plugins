package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/PastureStack/storage-plugins/internal/model"
)

const (
	ErrorInputTooLarge  = "input-too-large"
	ErrorInvalidJSON    = "invalid-json"
	ErrorInvalidRequest = "invalid-request"
)

type InputError struct {
	Code string
}

func (e *InputError) Error() string { return e.Code }

type rawRequest struct {
	APIVersion         string              `json:"apiVersion"`
	Kind               string              `json:"kind"`
	Driver             model.DriverID      `json:"driver"`
	Operation          model.Operation     `json:"operation"`
	IdempotencyKey     string              `json:"idempotencyKey"`
	OwnerID            string              `json:"ownerId"`
	ExpectedGeneration uint64              `json:"expectedGeneration"`
	ObservedState      model.ObservedState `json:"observedState"`
	Desired            rawDesired          `json:"desired"`
}

type rawDesired struct {
	Phase             model.Phase     `json:"phase"`
	AllowIrreversible bool            `json:"allowIrreversible"`
	Config            json.RawMessage `json:"config"`
}

var rootKeys = objectSchema{
	allowed: requiredSet(
		"apiVersion",
		"kind",
		"driver",
		"operation",
		"idempotencyKey",
		"ownerId",
		"expectedGeneration",
		"observedState",
		"desired",
	),
	required: requiredSet(
		"apiVersion",
		"kind",
		"driver",
		"operation",
		"idempotencyKey",
		"ownerId",
		"expectedGeneration",
		"observedState",
		"desired",
	),
}

var observedKeys = objectSchema{
	allowed:  requiredSet("phase", "generation", "ownerId", "resourceRef"),
	required: requiredSet("phase", "generation", "ownerId"),
}

var desiredKeys = objectSchema{
	allowed:  requiredSet("phase", "allowIrreversible", "config"),
	required: requiredSet("phase", "allowIrreversible", "config"),
}

type objectSchema struct {
	allowed  map[string]struct{}
	required map[string]struct{}
}

func requiredSet(keys ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		result[key] = struct{}{}
	}
	return result
}

func Decode(reader io.Reader) (model.Request, error) {
	data, err := io.ReadAll(io.LimitReader(reader, model.MaxInputSize+1))
	if err != nil {
		return model.Request{}, &InputError{Code: ErrorInvalidJSON}
	}
	if len(data) > model.MaxInputSize {
		return model.Request{}, &InputError{Code: ErrorInputTooLarge}
	}
	if len(bytes.TrimSpace(data)) == 0 || !utf8.Valid(data) {
		return model.Request{}, &InputError{Code: ErrorInvalidJSON}
	}
	if err := inspectJSON(data); err != nil {
		return model.Request{}, err
	}

	root, err := rawObject(data, rootKeys)
	if err != nil {
		return model.Request{}, err
	}
	if _, err := rawObject(root["observedState"], observedKeys); err != nil {
		return model.Request{}, err
	}
	desiredObject, err := rawObject(root["desired"], desiredKeys)
	if err != nil {
		return model.Request{}, err
	}

	var raw rawRequest
	if err := strictUnmarshal(data, &raw); err != nil {
		return model.Request{}, &InputError{Code: ErrorInvalidRequest}
	}
	config, err := decodeConfig(raw.Driver, desiredObject["config"])
	if err != nil {
		return model.Request{}, err
	}

	return model.Request{
		APIVersion:         raw.APIVersion,
		Kind:               raw.Kind,
		Driver:             raw.Driver,
		Operation:          raw.Operation,
		IdempotencyKey:     raw.IdempotencyKey,
		OwnerID:            raw.OwnerID,
		ExpectedGeneration: raw.ExpectedGeneration,
		ObservedState:      raw.ObservedState,
		Desired: model.Desired{
			Phase:             raw.Desired.Phase,
			AllowIrreversible: raw.Desired.AllowIrreversible,
			Config:            config,
		},
	}, nil
}

func inspectJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := inspectValue(decoder, 1); err != nil {
		return &InputError{Code: ErrorInvalidJSON}
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return &InputError{Code: ErrorInvalidJSON}
	}
	return nil
}

func inspectValue(decoder *json.Decoder, depth int) error {
	if depth > model.MaxJSONDepth {
		return errors.New("maximum depth exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || !safeText(key) {
					return errors.New("invalid object key")
				}
				if _, duplicate := seen[key]; duplicate {
					return errors.New("duplicate object key")
				}
				seen[key] = struct{}{}
				if err := inspectValue(decoder, depth+1); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("invalid object ending")
			}
		case '[':
			for decoder.More() {
				if err := inspectValue(decoder, depth+1); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("invalid array ending")
			}
		default:
			return errors.New("unexpected delimiter")
		}
	case string:
		if !safeText(value) {
			return errors.New("unsafe text")
		}
	case nil:
		return errors.New("null values are not allowed")
	case json.Number, bool:
		return nil
	default:
		return errors.New("unexpected token")
	}
	return nil
}

func safeText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func rawObject(data []byte, schema objectSchema) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, &InputError{Code: ErrorInvalidRequest}
	}
	for key := range object {
		if _, ok := schema.allowed[key]; !ok {
			return nil, &InputError{Code: ErrorInvalidRequest}
		}
	}
	for key := range schema.required {
		if _, ok := object[key]; !ok {
			return nil, &InputError{Code: ErrorInvalidRequest}
		}
	}
	return object, nil
}

func strictUnmarshal(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func decodeConfig(driver model.DriverID, data []byte) (model.DriverConfig, error) {
	switch driver {
	case model.DriverAliyunBlock:
		var config model.AliyunBlockConfig
		if err := decodeTypedConfig(data, &config, []string{"diskCategory", "sizeGiB", "snapshotRef"}, []string{"diskCategory", "sizeGiB"}); err != nil {
			return nil, err
		}
		return config, nil
	case model.DriverAWSEBS:
		var config model.AWSEBSConfig
		if err := decodeTypedConfig(data, &config, []string{"volumeType", "sizeGiB", "iops", "snapshotRef", "encrypted", "kmsRef"}, []string{"volumeType", "sizeGiB", "encrypted"}); err != nil {
			return nil, err
		}
		return config, nil
	case model.DriverAWSEFS:
		var config model.AWSEFSConfig
		if err := decodeTypedConfig(data, &config, []string{"performanceMode", "exportRef", "mountOptions"}, []string{"performanceMode", "exportRef"}); err != nil {
			return nil, err
		}
		sort.Strings(config.MountOptions)
		return config, nil
	case model.DriverNFS:
		var config model.NFSConfig
		if err := decodeTypedConfig(data, &config, []string{"serverRef", "exportRef", "mountOptions", "purgePolicy"}, []string{"serverRef", "exportRef", "purgePolicy"}); err != nil {
			return nil, err
		}
		sort.Strings(config.MountOptions)
		return config, nil
	case model.DriverCephRBD:
		var config model.CephRBDConfig
		if err := decodeTypedConfig(data, &config, []string{"pool", "sizeGiB", "imageFeature"}, []string{"pool", "sizeGiB", "imageFeature"}); err != nil {
			return nil, err
		}
		return config, nil
	case model.DriverLonghorn:
		var config model.LonghornConfig
		if err := decodeTypedConfig(data, &config, []string{"sizeBytes", "replicas"}, []string{"sizeBytes", "replicas"}); err != nil {
			return nil, err
		}
		return config, nil
	case model.DriverLoop:
		var config model.LoopConfig
		if err := decodeTypedConfig(data, &config, []string{"sizeMiB"}, []string{"sizeMiB"}); err != nil {
			return nil, err
		}
		return config, nil
	default:
		return nil, &InputError{Code: ErrorInvalidRequest}
	}
}

func decodeTypedConfig(data []byte, destination any, allowed, required []string) error {
	schema := objectSchema{allowed: requiredSet(allowed...), required: requiredSet(required...)}
	if _, err := rawObject(data, schema); err != nil {
		return err
	}
	if err := strictUnmarshal(data, destination); err != nil {
		return &InputError{Code: ErrorInvalidRequest}
	}
	return nil
}
