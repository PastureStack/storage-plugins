package planner

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/PastureStack/storage-plugins/internal/contract"
	"github.com/PastureStack/storage-plugins/internal/i18n"
	"github.com/PastureStack/storage-plugins/internal/model"
)

func TestPlanIsDeterministicAcrossEquivalentOptionOrder(t *testing.T) {
	first := decodeValid(t, nfsCreateJSON(`["sync","hard","nfs-v4.1"]`))
	second := decodeValid(t, nfsCreateJSON(`["nfs-v4.1","sync","hard"]`))
	messages := englishMessages(t)

	firstPlan, err := Build(first, messages)
	if err != nil {
		t.Fatal(err)
	}
	secondPlan, err := Build(second, messages)
	if err != nil {
		t.Fatal(err)
	}
	if firstPlan.PlanID != secondPlan.PlanID {
		t.Fatalf("equivalent requests produced different IDs: %q != %q", firstPlan.PlanID, secondPlan.PlanID)
	}
	firstJSON, _ := json.Marshal(firstPlan)
	secondJSON, _ := json.Marshal(secondPlan)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("equivalent plans differ\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestNullCannotCollideWithOmittedOptionalFieldPlanHash(t *testing.T) {
	omitted := `{"apiVersion":"storage-plugins.pasturestack.io/v1alpha1","kind":"LifecyclePlanRequest","driver":"aws-ebs","operation":"create","idempotencyKey":"request-ebs-omitted","ownerId":"team-a","expectedGeneration":0,"observedState":{"phase":"absent","generation":0,"ownerId":""},"desired":{"phase":"available","allowIrreversible":true,"config":{"volumeType":"gp3","sizeGiB":100,"iops":3000,"encrypted":true}}}`
	request := decodeValid(t, omitted)
	plan, err := Build(request, englishMessages(t))
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanID == "" {
		t.Fatal("omitted optional field did not produce a plan hash")
	}

	withNull := strings.Replace(omitted, `"encrypted":true`, `"encrypted":true,"kmsRef":null`, 1)
	_, err = contract.Decode(strings.NewReader(withNull))
	if err == nil {
		t.Fatal("null optional field reached canonical hashing")
	}
	var inputError *contract.InputError
	if !errors.As(err, &inputError) || inputError.Code != contract.ErrorInvalidJSON {
		t.Fatalf("null hash collision guard returned %v", err)
	}
}

func TestIrreversibleAcknowledgementNeverEnablesExecution(t *testing.T) {
	request := aliyunCreateRequest()
	messages := englishMessages(t)
	for _, acknowledged := range []bool{false, true} {
		request.Desired.AllowIrreversible = acknowledged
		plan, err := Build(request, messages)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Executable || plan.Effect != "none" {
			t.Fatalf("plan executed with acknowledgement=%v: %#v", acknowledged, plan)
		}
		if !reflect.DeepEqual(plan.Controls, model.Controls{}) {
			t.Fatalf("controls enabled with acknowledgement=%v: %#v", acknowledged, plan.Controls)
		}
		if len(plan.Steps) != 2 || !plan.Steps[1].Irreversible {
			t.Fatalf("expected blocked format step: %#v", plan.Steps)
		}
		for _, step := range plan.Steps {
			if step.Status != "blocked" || !strings.HasPrefix(step.Action, "would-") {
				t.Fatalf("step is not a blocked intent: %#v", step)
			}
		}
		gate := findGate(t, plan, "irreversible")
		if gate.Status != "blocked" {
			t.Fatalf("irreversible gate not blocked: %#v", gate)
		}
	}
}

func TestNFSPurgePolicyChangesOnlyTheBlockedIntent(t *testing.T) {
	messages := englishMessages(t)
	for _, test := range []struct {
		policy       string
		irreversible bool
		action       string
	}{
		{policy: "retain", irreversible: false, action: "would-retain-data-and-remove-record"},
		{policy: "purge-owned-subdirectory", irreversible: true, action: "would-remove-resource"},
	} {
		request := nfsRemoveRequest(test.policy)
		plan, err := Build(request, messages)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Steps) != 1 || plan.Steps[0].Action != test.action || plan.Steps[0].Irreversible != test.irreversible {
			t.Fatalf("policy %q produced %#v", test.policy, plan.Steps)
		}
		if plan.Steps[0].Status != "blocked" || plan.Executable {
			t.Fatalf("policy %q enabled execution", test.policy)
		}
	}
}

func TestDevelopmentDriverIsExplicitlyMarked(t *testing.T) {
	request := model.Request{
		APIVersion:         model.APIVersion,
		Kind:               model.RequestKind,
		Driver:             model.DriverLoop,
		Operation:          model.OperationCreate,
		IdempotencyKey:     "request-loop",
		OwnerID:            "team-a",
		ExpectedGeneration: 0,
		ObservedState: model.ObservedState{
			Phase:      model.PhaseAbsent,
			Generation: 0,
			OwnerID:    "",
		},
		Desired: model.Desired{
			Phase:  model.PhaseAvailable,
			Config: model.LoopConfig{SizeMiB: 64},
		},
	}
	plan, err := Build(request, englishMessages(t))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(plan, "development-only") {
		t.Fatalf("development diagnostic missing: %#v", plan.Diagnostics)
	}
}

func decodeValid(t *testing.T, input string) model.Request {
	t.Helper()
	request, err := contract.Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if issues := contract.Validate(request); len(issues) != 0 {
		t.Fatalf("invalid fixture: %#v", issues)
	}
	return request
}

func nfsCreateJSON(options string) string {
	return `{"apiVersion":"storage-plugins.pasturestack.io/v1alpha1","kind":"LifecyclePlanRequest","driver":"nfs","operation":"create","idempotencyKey":"request-nfs","ownerId":"team-a","expectedGeneration":0,"observedState":{"phase":"absent","generation":0,"ownerId":""},"desired":{"phase":"available","allowIrreversible":false,"config":{"serverRef":"ref:server-a","exportRef":"ref:export-a","mountOptions":` + options + `,"purgePolicy":"retain"}}}`
}

func aliyunCreateRequest() model.Request {
	return model.Request{
		APIVersion:         model.APIVersion,
		Kind:               model.RequestKind,
		Driver:             model.DriverAliyunBlock,
		Operation:          model.OperationCreate,
		IdempotencyKey:     "request-aliyun",
		OwnerID:            "team-a",
		ExpectedGeneration: 0,
		ObservedState: model.ObservedState{
			Phase:      model.PhaseAbsent,
			Generation: 0,
			OwnerID:    "",
		},
		Desired: model.Desired{
			Phase: model.PhaseAvailable,
			Config: model.AliyunBlockConfig{
				DiskCategory: "cloud-ssd",
				SizeGiB:      20,
			},
		},
	}
}

func nfsRemoveRequest(policy string) model.Request {
	return model.Request{
		APIVersion:         model.APIVersion,
		Kind:               model.RequestKind,
		Driver:             model.DriverNFS,
		Operation:          model.OperationRemove,
		IdempotencyKey:     "request-nfs-remove",
		OwnerID:            "team-a",
		ExpectedGeneration: 5,
		ObservedState: model.ObservedState{
			Phase:       model.PhaseAvailable,
			Generation:  5,
			OwnerID:     "team-a",
			ResourceRef: "ref:nfs-volume-a",
		},
		Desired: model.Desired{
			Phase: model.PhaseAbsent,
			Config: model.NFSConfig{
				ServerRef:   "ref:server-a",
				ExportRef:   "ref:export-a",
				PurgePolicy: policy,
			},
		},
	}
}

func englishMessages(t *testing.T) i18n.Catalog {
	t.Helper()
	messages, err := i18n.Load(i18n.EnglishUS)
	if err != nil {
		t.Fatal(err)
	}
	return messages
}

func findGate(t *testing.T, plan model.PlanOutput, name string) model.Gate {
	t.Helper()
	for _, gate := range plan.Gates {
		if gate.Name == name {
			return gate
		}
	}
	t.Fatalf("gate %q not found", name)
	return model.Gate{}
}

func hasDiagnostic(plan model.PlanOutput, code string) bool {
	for _, diagnostic := range plan.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
