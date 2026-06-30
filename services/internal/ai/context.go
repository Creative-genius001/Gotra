package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/gotra/gotra/internal/requests"
)

// ContextBuilder assembles a structured text context package from captured
// traffic for the prompt engine (Backend Bible — Context Builder).
type ContextBuilder struct {
	reqs *requests.Repository
}

// NewContextBuilder constructs a ContextBuilder.
func NewContextBuilder(reqs *requests.Repository) *ContextBuilder {
	return &ContextBuilder{reqs: reqs}
}

const maxBodyChars = 4000

// ForRequest builds a context block for a single captured request and returns
// the owning project, plus a completeness factor (1.0 when a response was
// captured, lower otherwise).
func (b *ContextBuilder) ForRequest(ctx context.Context, userID, requestID uuid.UUID) (block string, projectID uuid.UUID, completeness float64, err error) {
	d, err := b.reqs.GetForUser(ctx, userID, requestID)
	if err != nil {
		return "", uuid.Nil, 0, err
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "REQUEST\nMethod: %s\nPath: %s\n", d.Method, d.Path)
	if d.Query != "" {
		fmt.Fprintf(&sb, "Query: %s\n", d.Query)
	}
	sb.WriteString("Request headers:\n")
	writeHeaders(&sb, d.Headers)
	if d.Body != "" {
		fmt.Fprintf(&sb, "Request body:\n%s\n", truncate(d.Body))
	}

	completeness = 0.6
	if d.Response != nil {
		completeness = 1.0
		fmt.Fprintf(&sb, "\nRESPONSE\nStatus: %d\nDuration: %dms\n", d.Response.Status, d.Response.DurationMs)
		sb.WriteString("Response headers:\n")
		writeHeaders(&sb, d.Response.Headers)
		if d.Response.Body != "" {
			fmt.Fprintf(&sb, "Response body:\n%s\n", truncate(d.Response.Body))
		}
	} else {
		sb.WriteString("\nRESPONSE\n(no response was captured for this request)\n")
	}

	return sb.String(), d.ProjectID, completeness, nil
}

// ForRecentFailures builds an incident context from a project's recent non-2xx
// requests. Returns the block and the number of failing requests found.
func (b *ContextBuilder) ForRecentFailures(ctx context.Context, userID, projectID uuid.UUID) (string, int, error) {
	items, err := b.reqs.ListForProject(ctx, userID, projectID, 100)
	if err != nil {
		return "", 0, err
	}

	var sb strings.Builder
	sb.WriteString("RECENT FAILED REQUESTS (most recent first)\n")
	count := 0
	for _, it := range items {
		if it.Status == nil || *it.Status < 400 {
			continue
		}
		dur := 0
		if it.DurationMs != nil {
			dur = *it.DurationMs
		}
		fmt.Fprintf(&sb, "- %s %s → %d (%dms) at %s\n", it.Method, it.Path, *it.Status, dur, it.ReceivedAt.Format("15:04:05"))
		count++
		if count >= 25 {
			break
		}
	}
	return sb.String(), count, nil
}

func writeHeaders(sb *strings.Builder, headers map[string][]string) {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(sb, "  %s: %s\n", k, strings.Join(headers[k], ", "))
	}
}

func truncate(s string) string {
	if len(s) <= maxBodyChars {
		return s
	}
	return s[:maxBodyChars] + "\n…(truncated)"
}
