package contract

import (
	"regexp"
	"sort"

	"github.com/PastureStack/storage-plugins/internal/catalog"
	"github.com/PastureStack/storage-plugins/internal/model"
)

const maxSafeInteger = uint64(9_007_199_254_740_991)

var (
	safeIDPattern    = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._:-]{0,126}[a-z0-9])?$`)
	opaqueRefPattern = regexp.MustCompile(`^(?:ref:[a-z0-9][a-z0-9._:-]{0,120}|sha256:[0-9a-f]{64})$`)
)

type Issue struct {
	Severity string
	Code     string
}

var transitions = map[model.Operation]struct {
	from model.Phase
	to   model.Phase
}{
	model.OperationCreate:  {from: model.PhaseAbsent, to: model.PhaseAvailable},
	model.OperationAttach:  {from: model.PhaseAvailable, to: model.PhaseAttached},
	model.OperationMount:   {from: model.PhaseAttached, to: model.PhaseMounted},
	model.OperationUnmount: {from: model.PhaseMounted, to: model.PhaseAttached},
	model.OperationDetach:  {from: model.PhaseAttached, to: model.PhaseAvailable},
	model.OperationRemove:  {from: model.PhaseAvailable, to: model.PhaseAbsent},
}

func Validate(request model.Request) []Issue {
	issues := map[string]Issue{}
	add := func(code string) {
		issues[code] = Issue{Severity: "error", Code: code}
	}

	if request.APIVersion != model.APIVersion {
		add("invalid-api-version")
	}
	if request.Kind != model.RequestKind {
		add("invalid-kind")
	}
	if !safeIDPattern.MatchString(request.IdempotencyKey) || !safeIDPattern.MatchString(request.OwnerID) {
		add("invalid-identifier")
	}
	if request.ExpectedGeneration > maxSafeInteger || request.ObservedState.Generation > maxSafeInteger {
		add("invalid-generation")
	}
	if request.ExpectedGeneration != request.ObservedState.Generation {
		add("generation-mismatch")
	}

	capability, driverKnown := catalog.Lookup(request.Driver)
	if !driverKnown {
		add("unsupported-driver")
	} else if !catalog.SupportsOperation(capability, request.Operation) {
		add("unsupported-operation")
	}

	transition, operationKnown := transitions[request.Operation]
	if !operationKnown {
		add("unsupported-operation")
	} else if request.ObservedState.Phase != transition.from || request.Desired.Phase != transition.to {
		add("invalid-transition")
	}

	if request.Operation == model.OperationCreate {
		if request.ObservedState.OwnerID != "" || request.ObservedState.ResourceRef != "" {
			add("ownership-mismatch")
		}
	} else {
		if request.ObservedState.OwnerID != request.OwnerID {
			add("ownership-mismatch")
		}
		if !opaqueRefPattern.MatchString(request.ObservedState.ResourceRef) {
			add("unsafe-resource-reference")
		}
	}

	if request.Desired.Config == nil || request.Desired.Config.DriverID() != request.Driver {
		add("invalid-driver-config")
	} else {
		for _, code := range validateConfig(request.Desired.Config) {
			add(code)
		}
	}

	result := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, issue)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})
	return result
}

func ExpectedTransition(operation model.Operation) (model.Phase, model.Phase, bool) {
	transition, ok := transitions[operation]
	return transition.from, transition.to, ok
}

func validateConfig(config model.DriverConfig) []string {
	var issues []string
	add := func(condition bool, code string) {
		if condition {
			issues = append(issues, code)
		}
	}

	switch value := config.(type) {
	case model.AliyunBlockConfig:
		add(!oneOf(value.DiskCategory, "cloud", "cloud-efficiency", "cloud-ssd", "cloud-essd"), "invalid-disk-category")
		add(value.SizeGiB < 1 || value.SizeGiB > 32_768, "invalid-size")
		add(value.SnapshotRef != "" && !opaqueRefPattern.MatchString(value.SnapshotRef), "unsafe-snapshot-reference")
	case model.AWSEBSConfig:
		add(!oneOf(value.VolumeType, "gp2", "gp3", "io1", "io2", "sc1", "st1", "standard"), "invalid-volume-type")
		add(value.SizeGiB < 1 || value.SizeGiB > 65_536, "invalid-size")
		needsIOPS := value.VolumeType == "io1" || value.VolumeType == "io2"
		allowsIOPS := needsIOPS || value.VolumeType == "gp3"
		add(needsIOPS && value.IOPS == 0, "invalid-iops")
		add(value.IOPS != 0 && (!allowsIOPS || value.IOPS < 100 || value.IOPS > 256_000), "invalid-iops")
		add(value.SnapshotRef != "" && !opaqueRefPattern.MatchString(value.SnapshotRef), "unsafe-snapshot-reference")
		add(value.KMSRef != "" && (!value.Encrypted || !opaqueRefPattern.MatchString(value.KMSRef)), "unsafe-kms-reference")
	case model.AWSEFSConfig:
		add(!oneOf(value.PerformanceMode, "general-purpose", "max-io"), "invalid-performance-mode")
		add(!opaqueRefPattern.MatchString(value.ExportRef), "unsafe-export-reference")
		issues = append(issues, validateMountOptions(value.MountOptions)...)
	case model.NFSConfig:
		add(!opaqueRefPattern.MatchString(value.ServerRef), "unsafe-server-reference")
		add(!opaqueRefPattern.MatchString(value.ExportRef), "unsafe-export-reference")
		add(!oneOf(value.PurgePolicy, "retain", "purge-owned-subdirectory"), "invalid-purge-policy")
		issues = append(issues, validateMountOptions(value.MountOptions)...)
	case model.CephRBDConfig:
		add(!safeIDPattern.MatchString(value.Pool), "invalid-pool")
		add(value.SizeGiB < 1 || value.SizeGiB > 65_536, "invalid-size")
		add(!oneOf(value.ImageFeature, "layering", "exclusive-lock", "object-map", "fast-diff", "deep-flatten"), "invalid-image-feature")
	case model.LonghornConfig:
		add(value.SizeBytes < 1_048_576 || value.SizeBytes > 17_592_186_044_416 || value.SizeBytes%4096 != 0, "invalid-size")
		add(value.Replicas < 1 || value.Replicas > 5, "invalid-replica-count")
	case model.LoopConfig:
		add(value.SizeMiB < 1 || value.SizeMiB > 4096, "invalid-size")
	default:
		issues = append(issues, "invalid-driver-config")
	}

	sort.Strings(issues)
	return deduplicate(issues)
}

func validateMountOptions(options []string) []string {
	allowed := map[string]struct{}{
		"hard":      {},
		"nfs-v4.1":  {},
		"no-atime":  {},
		"read-only": {},
		"sync":      {},
	}
	seen := map[string]struct{}{}
	for _, option := range options {
		if _, ok := allowed[option]; !ok {
			return []string{"invalid-mount-option"}
		}
		if _, duplicate := seen[option]; duplicate {
			return []string{"duplicate-mount-option"}
		}
		seen[option] = struct{}{}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func deduplicate(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
