package flow

import (
	"context"
	"encoding/json"
	"html"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const prTimeout = 24 * time.Hour

type PRInput struct{}

type PullRequest struct {
	Body  string `json:"body" jsonschema:"Description as customer-facing release notes, a markdown list"`
	Title string `json:"title" jsonschema:"Title under 80 characters"`
}

func (e *Engine) createPR(ctx dbos.DBOSContext, _ PRInput) (string, error) {
	summary, err := act(ctx, "collect_changes", "Collect changes", "", func(stepCtx context.Context) (string, error) {
		return e.drafter.Context(stepCtx)
	})
	if err != nil {
		return "", errors.Wrap(err, "collect changes")
	}

	if _, err := dbos.RunAsStep(ctx, func(context.Context) (stepBegin, error) {
		return stepBegin{Input: summary, Name: "draft", Title: "Draft release notes"}, nil
	}, dbos.WithStepName("begin/draft")); err != nil {
		return "", errors.Wrap(err, "begin draft")
	}
	draft, err := dbos.RunAsStep(ctx, func(stepCtx context.Context) (PullRequest, error) {
		return e.drafter.Draft(stepCtx, summary)
	}, dbos.WithStepName("draft"))
	if err != nil {
		return "", errors.Wrap(err, "draft")
	}
	draft = normalizePullRequest(draft)

	pr, action, err := elicit.Form(ctx, "review", "Review the pull request", draft, prTimeout)
	if err != nil {
		return "", errors.Wrap(err, "review")
	}
	if action != elicit.ActionAccept {
		return "", nil
	}

	_, err = act(ctx, "respond/ready", "Pull request ready", pr.Title, func(context.Context) (string, error) {
		return "Drafted **" + pr.Title + "**\n\n" + pr.Body, nil
	})
	if err != nil {
		return "", errors.Wrap(err, "finalize")
	}
	return pr.Title, nil
}

type PRDrafter interface {
	Context(ctx context.Context) (string, error)
	Draft(ctx context.Context, summary string) (PullRequest, error)
}

func DefaultPRDrafter(workdir string) PRDrafter {
	return gitPRDrafter{workdir: workdir}
}

type gitPRDrafter struct {
	workdir string
}

func (d gitPRDrafter) Context(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	log := d.git(ctx, "log", "--oneline", "origin/main..HEAD")
	stat := d.git(ctx, "diff", "--stat", "origin/main...HEAD")
	summary := "Commits:\n" + strings.TrimSpace(log)
	if strings.TrimSpace(stat) != "" {
		summary += "\n\nChanges:\n" + strings.TrimSpace(stat)
	}
	return summary, nil
}

func (d gitPRDrafter) git(ctx context.Context, args ...string) string {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", d.workdir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

const draftPrompt = `Write a pull request title and description for the changes below.
The title must be under 80 characters. The description must be customer-facing release notes as a markdown list — every line starts with "- ". Summarize user-facing capability, not implementation.

Changes:
`

func (d gitPRDrafter) Draft(ctx context.Context, summary string) (PullRequest, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return heuristicPR(summary), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	schema, err := jsonschema.For[PullRequest](nil)
	if err != nil {
		return heuristicPR(summary), nil
	}
	schema.Required = nil
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return heuristicPR(summary), nil
	}
	out, err := exec.CommandContext(ctx, "claude",
		"-p", draftPrompt+summary,
		"--model", "sonnet",
		"--output-format", "json",
		"--json-schema", string(schemaJSON),
	).Output()
	if err != nil {
		return heuristicPR(summary), nil
	}
	var result struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(out, &result); err != nil || result.IsError {
		return heuristicPR(summary), nil
	}
	var pr PullRequest
	if err := json.Unmarshal([]byte(result.Result), &pr); err != nil || pr.Title == "" {
		return heuristicPR(summary), nil
	}
	return pr, nil
}

func heuristicPR(summary string) PullRequest {
	lines := []string{}
	for _, line := range strings.Split(summary, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		lines = append(lines, line)
	}
	title := "Update"
	body := []string{}
	for i, line := range lines {
		if i == 0 {
			title = commitSubject(line)
		}
		body = append(body, "- "+commitSubject(line))
	}
	return PullRequest{Body: strings.Join(body, "\n"), Title: title}
}

func normalizePullRequest(pr PullRequest) PullRequest {
	pr.Body = html.UnescapeString(pr.Body)
	pr.Title = html.UnescapeString(pr.Title)
	return pr
}

func commitSubject(line string) string {
	if _, rest, ok := strings.Cut(line, " "); ok {
		if len(rest) > 0 {
			return rest
		}
	}
	return line
}
