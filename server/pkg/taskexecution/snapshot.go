// Package taskexecution defines daemon-observed execution snapshots. These are
// wall-clock orchestration phases, not provider-only model generation timings.
package taskexecution

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

const MaxPayloadBytes = 16 * 1024

const (
	Prepare   = "prepare"
	Execute   = "execute"
	Finalize  = "finalize"
	Running   = "running"
	Completed = "completed"
	Failed    = "failed"
	Cancelled = "cancelled"
)

type Phase struct {
	Name       string    `json:"name"`
	StartedAt  time.Time `json:"started_at"`
	DurationMS int64     `json:"duration_ms"`
	Status     string    `json:"status"`
}

type Snapshot struct {
	SchemaVersion        int        `json:"schema_version"`
	Provider             string     `json:"provider"`
	RequestedModel       string     `json:"requested_model"`
	DaemonVersion        string     `json:"daemon_version"`
	DaemonCommit         string     `json:"daemon_commit"`
	CommunityBaseVersion string     `json:"community_base_version"`
	DirectAgentMode      bool       `json:"direct_agent_mode"`
	ConciseMode          bool       `json:"concise_mode"`
	StartedAt            time.Time  `json:"started_at"`
	FinishedAt           *time.Time `json:"finished_at,omitempty"`
	Phases               []Phase    `json:"phases"`
	ToolCalls            *int64     `json:"tool_calls,omitempty"`
}

// Decode rejects unknown/missing/null fields rather than silently turning an
// absent historical mode or duration into false/zero. The same decoder makes
// malformed or unsupported stored snapshots disappear safely from responses.
func Decode(data []byte) (*Snapshot, error) {
	if len(data) > MaxPayloadBytes {
		return nil, errors.New("execution snapshot is too large")
	}
	var s Snapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("expected one execution snapshot")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, key := range []string{"schema_version", "provider", "requested_model", "daemon_version", "daemon_commit", "community_base_version", "direct_agent_mode", "concise_mode", "started_at", "phases"} {
		if value, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("missing execution field %s", key)
		}
	}
	var phases []map[string]json.RawMessage
	if err := json.Unmarshal(fields["phases"], &phases); err != nil {
		return nil, err
	}
	for _, phase := range phases {
		for _, key := range []string{"name", "started_at", "duration_ms", "status"} {
			if value, ok := phase[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("missing phase field %s", key)
			}
		}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != 1 || s.StartedAt.IsZero() || len(s.Phases) < 1 || len(s.Phases) > 3 {
		return errors.New("invalid execution version, start time, or phase count")
	}
	if s.Provider == "" || !boundedText(s.Provider, 64) || !boundedText(s.RequestedModel, 256) ||
		!boundedText(s.DaemonVersion, 128) || !boundedText(s.DaemonCommit, 128) || !boundedText(s.CommunityBaseVersion, 128) {
		return errors.New("invalid execution metadata")
	}
	if s.ToolCalls != nil && (*s.ToolCalls < 0 || *s.ToolCalls > 1_000_000_000) {
		return errors.New("invalid tool call count")
	}
	if !s.Phases[0].StartedAt.Equal(s.StartedAt) || s.Phases[0].Name != Prepare {
		return errors.New("execution must start with prepare")
	}
	previousOrder := -1
	for i, phase := range s.Phases {
		order := phaseOrder(phase.Name)
		if order <= previousOrder || phase.StartedAt.IsZero() || phase.DurationMS < 0 || phase.DurationMS > int64((365*24*time.Hour)/time.Millisecond) {
			return errors.New("invalid phase order, timestamp, or duration")
		}
		previousOrder = order
		if phase.Status != Running && phase.Status != Completed && phase.Status != Failed && phase.Status != Cancelled {
			return errors.New("invalid phase status")
		}
		if i < len(s.Phases)-1 {
			next := s.Phases[i+1]
			if next.Name == Execute && phase.Status != Completed {
				return errors.New("execute requires completed preparation")
			}
			if phase.Status == Running || phase.StartedAt.After(next.StartedAt) || phase.StartedAt.Add(time.Duration(phase.DurationMS)*time.Millisecond).After(next.StartedAt) {
				return errors.New("overlapping or unfinished earlier phase")
			}
		}
	}
	last := s.Phases[len(s.Phases)-1]
	if s.FinishedAt == nil {
		if last.Status != Running {
			return errors.New("unfinished snapshot must have a running phase")
		}
	} else if last.Status == Running || s.FinishedAt.Before(last.StartedAt.Add(time.Duration(last.DurationMS)*time.Millisecond)) {
		return errors.New("invalid execution finish time")
	}
	return nil
}

func boundedText(value string, limit int) bool {
	return len(value) <= limit && !strings.ContainsFunc(value, unicode.IsControl)
}

func phaseOrder(name string) int {
	switch name {
	case Prepare:
		return 0
	case Execute:
		return 1
	case Finalize:
		return 2
	default:
		return -1
	}
}

// CanReplace is a monotonic update check, independent of JSON key ordering.
// Invalid legacy JSON is handled by Decode before this comparison. A newer
// observed start may replace a reclaimed execution, but an older one never can.
func (s Snapshot) CanReplace(previous Snapshot) bool {
	if !s.StartedAt.Equal(previous.StartedAt) {
		return s.StartedAt.After(previous.StartedAt)
	}
	if s.Provider != previous.Provider || s.RequestedModel != previous.RequestedModel ||
		s.DaemonVersion != previous.DaemonVersion || s.DaemonCommit != previous.DaemonCommit ||
		s.CommunityBaseVersion != previous.CommunityBaseVersion || s.DirectAgentMode != previous.DirectAgentMode ||
		s.ConciseMode != previous.ConciseMode || len(s.Phases) < len(previous.Phases) {
		return false
	}
	if previous.FinishedAt != nil && (s.FinishedAt == nil || !s.FinishedAt.Equal(*previous.FinishedAt) || len(s.Phases) != len(previous.Phases)) {
		return false
	}
	if previous.ToolCalls != nil && (s.ToolCalls == nil || *s.ToolCalls < *previous.ToolCalls) {
		return false
	}
	for i, before := range previous.Phases {
		after := s.Phases[i]
		if before.Name != after.Name || !before.StartedAt.Equal(after.StartedAt) || after.DurationMS < before.DurationMS {
			return false
		}
		if before.Status != Running && (after.Status != before.Status || after.DurationMS != before.DurationMS) {
			return false
		}
	}
	return true
}
