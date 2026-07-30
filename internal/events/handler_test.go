package events

import (
	"testing"

	"github.com/navikt/ghep/internal/github"
)

func TestEventBranch(t *testing.T) {
	tests := []struct {
		name      string
		event     github.Event
		eventType github.EventType
		want      string
	}{
		{
			name:      "commit on main",
			eventType: github.TypeCommit,
			event:     github.Event{Ref: "refs/heads/main"},
			want:      "main",
		},
		{
			name:      "commit on feature branch",
			eventType: github.TypeCommit,
			event:     github.Event{Ref: "refs/heads/feature/my-feature"},
			want:      "feature/my-feature",
		},
		{
			name:      "workflow on main",
			eventType: github.TypeWorkflow,
			event:     github.Event{Workflow: &github.Workflow{HeadBranch: "main"}},
			want:      "main",
		},
		{
			name:      "workflow with no workflow data",
			eventType: github.TypeWorkflow,
			event:     github.Event{},
			want:      "",
		},
		{
			name:      "pull request targeting main",
			eventType: github.TypePullRequest,
			event:     github.Event{PullRequest: &github.Issue{Base: github.IssueBase{Ref: "main"}}},
			want:      "main",
		},
		{
			name:      "pull request with no PR data",
			eventType: github.TypePullRequest,
			event:     github.Event{},
			want:      "",
		},
		{
			name:      "issue has no branch",
			eventType: github.TypeIssue,
			event:     github.Event{},
			want:      "",
		},
		{
			name:      "release has no branch",
			eventType: github.TypeRelease,
			event:     github.Event{},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eventBranch(tt.event, tt.eventType)
			if got != tt.want {
				t.Errorf("eventBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}
