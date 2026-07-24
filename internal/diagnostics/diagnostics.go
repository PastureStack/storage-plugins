package diagnostics

import (
	"sort"

	"github.com/PastureStack/storage-plugins/internal/contract"
	"github.com/PastureStack/storage-plugins/internal/i18n"
	"github.com/PastureStack/storage-plugins/internal/model"
)

func FromIssues(issues []contract.Issue, messages i18n.Catalog) []model.Diagnostic {
	result := make([]model.Diagnostic, 0, len(issues))
	for _, issue := range issues {
		result = append(result, model.Diagnostic{
			Severity: issue.Severity,
			Code:     issue.Code,
			Message:  messages.Message(issue.Code),
		})
	}
	sortDiagnostics(result)
	return result
}

func Add(result []model.Diagnostic, severity, code string, messages i18n.Catalog) []model.Diagnostic {
	result = append(result, model.Diagnostic{
		Severity: severity,
		Code:     code,
		Message:  messages.Message(code),
	})
	sortDiagnostics(result)
	return result
}

func sortDiagnostics(result []model.Diagnostic) {
	sort.Slice(result, func(i, j int) bool {
		if result[i].Code == result[j].Code {
			return result[i].Severity < result[j].Severity
		}
		return result[i].Code < result[j].Code
	})
}
