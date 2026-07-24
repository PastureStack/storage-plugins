package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/PastureStack/storage-plugins/internal/catalog"
	"github.com/PastureStack/storage-plugins/internal/diagnostics"
	"github.com/PastureStack/storage-plugins/internal/i18n"
	"github.com/PastureStack/storage-plugins/internal/model"
)

func Build(request model.Request, messages i18n.Catalog) (model.PlanOutput, error) {
	canonical, err := request.CanonicalJSON()
	if err != nil {
		return model.PlanOutput{}, err
	}
	digest := sha256.Sum256(canonical)
	capability, _ := catalog.Lookup(request.Driver)
	steps := buildSteps(request, capability)
	irreversible := false
	for _, step := range steps {
		if step.Irreversible {
			irreversible = true
			break
		}
	}

	planDiagnostics := []model.Diagnostic{}
	planDiagnostics = diagnostics.Add(planDiagnostics, "info", "execution-disabled", messages)
	if irreversible {
		planDiagnostics = diagnostics.Add(planDiagnostics, "warning", "irreversible-blocked", messages)
	}
	if capability.DevelopmentOnly {
		planDiagnostics = diagnostics.Add(planDiagnostics, "info", "development-only", messages)
	}

	return model.PlanOutput{
		APIVersion:     model.APIVersion,
		Kind:           "LifecyclePlan",
		Locale:         messages.Locale(),
		PlanID:         "sha256:" + hex.EncodeToString(digest[:]),
		Driver:         request.Driver,
		Operation:      request.Operation,
		ObservedPhase:  request.ObservedState.Phase,
		RequestedPhase: request.Desired.Phase,
		ValidIntent:    true,
		Executable:     false,
		Effect:         "none",
		Steps:          steps,
		Gates:          buildGates(request.Desired.AllowIrreversible, irreversible),
		Diagnostics:    planDiagnostics,
		Controls:       model.DisabledControls(),
	}, nil
}

func buildSteps(request model.Request, capability model.DriverCapability) []model.PlanStep {
	var definitions []struct {
		action       string
		irreversible bool
		requirements []string
	}

	switch request.Operation {
	case model.OperationCreate:
		definitions = append(definitions, struct {
			action       string
			irreversible bool
			requirements []string
		}{action: "would-create-resource", requirements: createRequirements(capability)})
		if capability.Traits.FilesystemPreparation {
			definitions = append(definitions, struct {
				action       string
				irreversible bool
				requirements []string
			}{action: "would-format-filesystem", irreversible: true, requirements: []string{"format", "host-path", "privileged"}})
		}
	case model.OperationAttach:
		action := "would-attach-resource"
		requirements := selectRequirements(capability.WouldRequire, "cloud-api", "host-path", "host-socket", "network", "privileged", "state-write")
		if capability.Traits.AttachMode == "logical" {
			action = "would-record-logical-attachment"
			requirements = []string{"state-write"}
		}
		definitions = append(definitions, struct {
			action       string
			irreversible bool
			requirements []string
		}{action: action, requirements: requirements})
	case model.OperationMount:
		definitions = append(definitions, struct {
			action       string
			irreversible bool
			requirements []string
		}{action: "would-mount-resource", requirements: selectRequirements(capability.WouldRequire, "host-path", "mount", "network", "privileged")})
	case model.OperationUnmount:
		definitions = append(definitions, struct {
			action       string
			irreversible bool
			requirements []string
		}{action: "would-unmount-resource", requirements: selectRequirements(capability.WouldRequire, "host-path", "network", "privileged", "unmount")})
	case model.OperationDetach:
		action := "would-detach-resource"
		requirements := selectRequirements(capability.WouldRequire, "cloud-api", "host-path", "host-socket", "network", "privileged", "state-write")
		if capability.Traits.AttachMode == "logical" {
			action = "would-record-logical-detachment"
			requirements = []string{"state-write"}
		}
		definitions = append(definitions, struct {
			action       string
			irreversible bool
			requirements []string
		}{action: action, requirements: requirements})
	case model.OperationRemove:
		action := "would-remove-resource"
		irreversible := true
		if nfs, ok := request.Desired.Config.(model.NFSConfig); ok && nfs.PurgePolicy == "retain" {
			action = "would-retain-data-and-remove-record"
			irreversible = false
		}
		definitions = append(definitions, struct {
			action       string
			irreversible bool
			requirements []string
		}{action: action, irreversible: irreversible, requirements: selectRequirements(capability.WouldRequire, "cloud-api", "delete", "host-path", "host-socket", "network", "privileged", "state-write")})
	}

	steps := make([]model.PlanStep, 0, len(definitions))
	for index, definition := range definitions {
		requirements := append([]string(nil), definition.requirements...)
		sort.Strings(requirements)
		steps = append(steps, model.PlanStep{
			Sequence:     index + 1,
			Action:       definition.action,
			Status:       "blocked",
			Irreversible: definition.irreversible,
			WouldRequire: requirements,
			ReasonCode:   "execution-disabled",
		})
	}
	return steps
}

func createRequirements(capability model.DriverCapability) []string {
	return selectRequirements(capability.WouldRequire, "cloud-api", "credential-read", "host-path", "host-socket", "network", "privileged", "state-write")
}

func selectRequirements(available []string, wanted ...string) []string {
	wantedSet := map[string]struct{}{}
	for _, item := range wanted {
		wantedSet[item] = struct{}{}
	}
	result := make([]string, 0, len(available))
	for _, item := range available {
		if _, ok := wantedSet[item]; ok {
			result = append(result, item)
		}
	}
	return result
}

func buildGates(acknowledged, irreversible bool) []model.Gate {
	irreversibleStatus := model.Gate{Name: "irreversible", Status: "not-required", Code: "not-required"}
	if irreversible {
		irreversibleStatus.Status = "blocked"
		irreversibleStatus.Code = "irreversible-blocked"
		if acknowledged {
			irreversibleStatus.Code = "acknowledged-execution-disabled"
		}
	}
	gates := []model.Gate{
		{Name: "capability", Status: "passed", Code: "capability-validated"},
		{Name: "execution", Status: "blocked", Code: "execution-disabled"},
		{Name: "generation", Status: "passed", Code: "generation-matched"},
		{Name: "idempotency", Status: "passed", Code: "deterministic-plan-id"},
		irreversibleStatus,
		{Name: "ownership", Status: "passed", Code: "ownership-matched"},
		{Name: "schema", Status: "passed", Code: "schema-validated"},
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].Name < gates[j].Name })
	return gates
}
