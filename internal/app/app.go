package app

import (
	"encoding/json"
	"errors"
	"flag"
	"io"

	"github.com/PastureStack/storage-plugins/internal/catalog"
	"github.com/PastureStack/storage-plugins/internal/contract"
	"github.com/PastureStack/storage-plugins/internal/diagnostics"
	"github.com/PastureStack/storage-plugins/internal/i18n"
	"github.com/PastureStack/storage-plugins/internal/model"
	"github.com/PastureStack/storage-plugins/internal/planner"
)

const (
	exitSuccess = 0
	exitInvalid = 2
	exitFailure = 1
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	defaultMessages, err := i18n.Load(i18n.EnglishUS)
	if err != nil {
		return exitFailure
	}
	if len(args) == 0 {
		writeError(stderr, defaultMessages, "unknown-command")
		return exitInvalid
	}

	command := args[0]
	locale, parseCode := parseLocale(args[1:])
	if parseCode != "" {
		writeError(stderr, defaultMessages, parseCode)
		return exitInvalid
	}
	messages, err := i18n.Load(locale)
	if err != nil {
		writeError(stderr, defaultMessages, "unsupported-locale")
		return exitInvalid
	}

	switch command {
	case "capabilities":
		if err := writeJSON(stdout, catalog.Output(locale)); err != nil {
			writeError(stderr, messages, "internal-error")
			return exitFailure
		}
		return exitSuccess
	case "validate", "plan":
		request, err := contract.Decode(stdin)
		if err != nil {
			writeError(stderr, messages, inputErrorCode(err))
			return exitInvalid
		}
		issues := contract.Validate(request)
		requestDiagnostics := diagnostics.FromIssues(issues, messages)
		requestDiagnostics = diagnostics.Add(requestDiagnostics, "info", "execution-disabled", messages)
		if command == "validate" {
			output := model.ValidationOutput{
				APIVersion:  model.APIVersion,
				Kind:        "LifecycleValidation",
				Locale:      locale,
				Valid:       len(issues) == 0,
				Driver:      request.Driver,
				Operation:   request.Operation,
				Diagnostics: requestDiagnostics,
				Controls:    model.DisabledControls(),
			}
			if err := writeJSON(stdout, output); err != nil {
				writeError(stderr, messages, "internal-error")
				return exitFailure
			}
			if len(issues) != 0 {
				return exitInvalid
			}
			return exitSuccess
		}

		if len(issues) != 0 {
			output := invalidPlan(request, requestDiagnostics, locale)
			if err := writeJSON(stdout, output); err != nil {
				writeError(stderr, messages, "internal-error")
				return exitFailure
			}
			return exitInvalid
		}
		output, err := planner.Build(request, messages)
		if err != nil {
			writeError(stderr, messages, "internal-error")
			return exitFailure
		}
		if err := writeJSON(stdout, output); err != nil {
			writeError(stderr, messages, "internal-error")
			return exitFailure
		}
		return exitSuccess
	default:
		writeError(stderr, messages, "unknown-command")
		return exitInvalid
	}
}

func parseLocale(args []string) (string, string) {
	flags := flag.NewFlagSet("storage-plugins", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	locale := flags.String("locale", i18n.EnglishUS, "output locale")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return "", "invalid-arguments"
	}
	if *locale != i18n.EnglishUS && *locale != i18n.TraditionalChinese {
		return "", "unsupported-locale"
	}
	return *locale, ""
}

func inputErrorCode(err error) string {
	var inputError *contract.InputError
	if errors.As(err, &inputError) {
		return inputError.Code
	}
	return "invalid-input"
}

func invalidPlan(request model.Request, requestDiagnostics []model.Diagnostic, locale string) model.PlanOutput {
	return model.PlanOutput{
		APIVersion:     model.APIVersion,
		Kind:           "LifecyclePlan",
		Locale:         locale,
		PlanID:         "",
		Driver:         request.Driver,
		Operation:      request.Operation,
		ObservedPhase:  request.ObservedState.Phase,
		RequestedPhase: request.Desired.Phase,
		ValidIntent:    false,
		Executable:     false,
		Effect:         "none",
		Steps:          []model.PlanStep{},
		Gates: []model.Gate{
			{Name: "execution", Status: "blocked", Code: "execution-disabled"},
			{Name: "schema", Status: "blocked", Code: "invalid-input"},
		},
		Diagnostics: requestDiagnostics,
		Controls:    model.DisabledControls(),
	}
}

func writeError(writer io.Writer, messages i18n.Catalog, code string) {
	_ = writeJSON(writer, model.ErrorOutput{
		APIVersion: model.APIVersion,
		Kind:       "Error",
		Code:       code,
		Message:    messages.Message(code),
	})
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
