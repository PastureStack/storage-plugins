package contract

import (
	"strings"
	"testing"

	"github.com/PastureStack/storage-plugins/internal/model"
)

func TestAllSevenDriverConfigsValidate(t *testing.T) {
	tests := []struct {
		driver string
		config string
	}{
		{driver: "aliyun-block", config: `{"diskCategory":"cloud-ssd","sizeGiB":20,"snapshotRef":"ref:snapshot-a"}`},
		{driver: "aws-ebs", config: `{"volumeType":"gp3","sizeGiB":100,"iops":3000,"snapshotRef":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","encrypted":true,"kmsRef":"ref:kms-policy-a"}`},
		{driver: "aws-efs", config: `{"performanceMode":"general-purpose","exportRef":"ref:export-a","mountOptions":["hard","nfs-v4.1"]}`},
		{driver: "nfs", config: `{"serverRef":"ref:server-a","exportRef":"ref:export-a","mountOptions":["read-only","sync"],"purgePolicy":"retain"}`},
		{driver: "ceph-rbd", config: `{"pool":"pool-a","sizeGiB":50,"imageFeature":"layering"}`},
		{driver: "longhorn", config: `{"sizeBytes":10737418240,"replicas":3}`},
		{driver: "loop", config: `{"sizeMiB":128}`},
	}

	for _, test := range tests {
		t.Run(test.driver, func(t *testing.T) {
			request, err := Decode(strings.NewReader(requestJSON(test.driver, test.config)))
			if err != nil {
				t.Fatal(err)
			}
			if issues := Validate(request); len(issues) != 0 {
				t.Fatalf("unexpected issues: %#v", issues)
			}
		})
	}
}

func TestLifecycleStateTable(t *testing.T) {
	tests := []struct {
		operation model.Operation
		from      model.Phase
		to        model.Phase
	}{
		{operation: model.OperationCreate, from: model.PhaseAbsent, to: model.PhaseAvailable},
		{operation: model.OperationAttach, from: model.PhaseAvailable, to: model.PhaseAttached},
		{operation: model.OperationMount, from: model.PhaseAttached, to: model.PhaseMounted},
		{operation: model.OperationUnmount, from: model.PhaseMounted, to: model.PhaseAttached},
		{operation: model.OperationDetach, from: model.PhaseAttached, to: model.PhaseAvailable},
		{operation: model.OperationRemove, from: model.PhaseAvailable, to: model.PhaseAbsent},
	}
	for _, test := range tests {
		t.Run(string(test.operation), func(t *testing.T) {
			request := lifecycleRequest(test.operation, test.from, test.to)
			if issues := Validate(request); len(issues) != 0 {
				t.Fatalf("unexpected issues: %#v", issues)
			}
			request.Desired.Phase = model.PhaseAbsent
			if request.Desired.Phase == test.to {
				request.Desired.Phase = model.PhaseMounted
			}
			if !hasIssue(Validate(request), "invalid-transition") {
				t.Fatalf("invalid transition was not rejected: %#v", request)
			}
		})
	}
}

func TestOwnershipGenerationAndIdempotencyGates(t *testing.T) {
	base := lifecycleRequest(model.OperationAttach, model.PhaseAvailable, model.PhaseAttached)

	ownerMismatch := base
	ownerMismatch.ObservedState.OwnerID = "team-b"
	if !hasIssue(Validate(ownerMismatch), "ownership-mismatch") {
		t.Fatal("ownership mismatch was not rejected")
	}

	generationMismatch := base
	generationMismatch.ExpectedGeneration++
	if !hasIssue(Validate(generationMismatch), "generation-mismatch") {
		t.Fatal("generation mismatch was not rejected")
	}

	unsafeKey := base
	unsafeKey.IdempotencyKey = "Request With Spaces"
	if !hasIssue(Validate(unsafeKey), "invalid-identifier") {
		t.Fatal("unsafe idempotency key was not rejected")
	}

	unsafeReference := base
	unsafeReference.ObservedState.ResourceRef = "https://storage.invalid/volume"
	if !hasIssue(Validate(unsafeReference), "unsafe-resource-reference") {
		t.Fatal("URL resource reference was not rejected")
	}
}

func TestUnsafeExternalReferencesAndMountOptionsAreRejected(t *testing.T) {
	nfs := requestJSON("nfs", `{"serverRef":"https://nfs.invalid","exportRef":"/srv/export","mountOptions":["rw,exec"],"purgePolicy":"purge-root"}`)
	request, err := Decode(strings.NewReader(nfs))
	if err != nil {
		t.Fatal(err)
	}
	issues := Validate(request)
	for _, code := range []string{"unsafe-server-reference", "unsafe-export-reference", "invalid-mount-option", "invalid-purge-policy"} {
		if !hasIssue(issues, code) {
			t.Fatalf("missing issue %q in %#v", code, issues)
		}
	}

	ebs := requestJSON("aws-ebs", `{"volumeType":"gp3","sizeGiB":100,"encrypted":true,"kmsRef":"arn:aws:kms:region:account:key/raw"}`)
	request, err = Decode(strings.NewReader(ebs))
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(Validate(request), "unsafe-kms-reference") {
		t.Fatal("raw KMS material reference was not rejected")
	}
}

func TestLoopMountCapabilityIsBlocked(t *testing.T) {
	request := model.Request{
		APIVersion:         model.APIVersion,
		Kind:               model.RequestKind,
		Driver:             model.DriverLoop,
		Operation:          model.OperationMount,
		IdempotencyKey:     "request-loop",
		OwnerID:            "team-a",
		ExpectedGeneration: 2,
		ObservedState: model.ObservedState{
			Phase:       model.PhaseAttached,
			Generation:  2,
			OwnerID:     "team-a",
			ResourceRef: "ref:loop-a",
		},
		Desired: model.Desired{
			Phase:  model.PhaseMounted,
			Config: model.LoopConfig{SizeMiB: 128},
		},
	}
	if !hasIssue(Validate(request), "unsupported-operation") {
		t.Fatal("loop mount capability was not blocked")
	}
}

func lifecycleRequest(operation model.Operation, from, to model.Phase) model.Request {
	request := model.Request{
		APIVersion:         model.APIVersion,
		Kind:               model.RequestKind,
		Driver:             model.DriverAliyunBlock,
		Operation:          operation,
		IdempotencyKey:     "request-001",
		OwnerID:            "team-a",
		ExpectedGeneration: 3,
		ObservedState: model.ObservedState{
			Phase:       from,
			Generation:  3,
			OwnerID:     "team-a",
			ResourceRef: "ref:volume-a",
		},
		Desired: model.Desired{
			Phase:  to,
			Config: model.AliyunBlockConfig{DiskCategory: "cloud-ssd", SizeGiB: 20},
		},
	}
	if operation == model.OperationCreate {
		request.ObservedState.OwnerID = ""
		request.ObservedState.ResourceRef = ""
	}
	return request
}

func hasIssue(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
