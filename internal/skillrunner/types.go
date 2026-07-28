package skillrunner

import (
	"encoding/json"
	"time"
)

type SkillRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Invocation struct {
	InvocationID     string          `json:"invocation_id"`
	ContractVersion  string          `json:"contract_version"`
	Skill            SkillRef        `json:"skill"`
	Input            json.RawMessage `json:"input"`
	AllowedResources []string        `json:"allowed_resources"`
	Deadline         time.Time       `json:"deadline"`
}

type Status string

const (
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Result struct {
	InvocationID string          `json:"invocation_id"`
	Status       Status          `json:"status"`
	Output       json.RawMessage `json:"output"`
	EvidenceRefs []string        `json:"evidence_refs"`
	Warnings     []string        `json:"warnings"`
	Metrics      map[string]any  `json:"metrics"`
	Error        *ResultError    `json:"error,omitempty"`
}

type ResultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
