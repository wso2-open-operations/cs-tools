package notify

import (
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
)

func TestRegressionCaseLink(t *testing.T) {
	row := regressionreport.Row{
		ID: "11111111-2222-4333-8444-555555555555", Number: "CS0000001", IssueKey: "EXAMPLESUB-1",
	}

	got := regressionCaseLink("https://portal.example.com/cases/", row)
	if !strings.Contains(got, `href="https://portal.example.com/cases/11111111-2222-4333-8444-555555555555"`) ||
		!strings.Contains(got, ">EXAMPLESUB-1</a>") {
		t.Errorf("linked cell = %s, want a link to the case id labelled with the issue key", got)
	}

	if got := regressionCaseLink("", row); got != "EXAMPLESUB-1" {
		t.Errorf("no base = %q, want the plain issue key", got)
	}
	if got := regressionCaseLink("https://portal.example.com/cases", regressionreport.Row{Number: "CS0000001"}); got != "CS0000001" {
		t.Errorf("no id = %q, want the plain number and no link", got)
	}
}
