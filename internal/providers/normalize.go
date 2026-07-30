package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type Transcript struct {
	Segments []TranscriptSegment `json:"segments"`
}

type TranscriptSegment struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Text         string  `json:"text"`
	SourceRef    string  `json:"source_ref"`
}

type KnowledgeResult struct {
	Entries []KnowledgeEntry `json:"entries"`
}

type KnowledgeEntry struct {
	ReferenceID string `json:"reference_id"`
	Text        string `json:"text"`
	Source      string `json:"source"`
	YearFrom    int    `json:"year_from"`
	YearTo      int    `json:"year_to"`
	Region      string `json:"region"`
	Confidence  string `json:"confidence"`
	License     string `json:"license"`
}

type MaterialAssessment struct {
	Complete     bool          `json:"complete"`
	CoveredItems []CoveredItem `json:"covered_items"`
	Gaps         []MaterialGap `json:"gaps"`
}

type CoveredItem struct {
	PlanItemID   string   `json:"plan_item_id"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type MaterialGap struct {
	PlanItemID string `json:"plan_item_id"`
	Reason     string `json:"reason"`
}

type FollowupPlan struct {
	Summary   string             `json:"summary"`
	Questions []FollowupQuestion `json:"questions"`
}

type FollowupQuestion struct {
	PlanItemID string `json:"plan_item_id"`
	Question   string `json:"question"`
}

func Normalize(task TaskType, raw json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%w: malformed JSON", ErrInvalidOutput)
	}
	switch task {
	case TaskAudioTranscription:
		var value Transcript
		if err := json.Unmarshal(raw, &value); err != nil || len(value.Segments) == 0 {
			return nil, fmt.Errorf("%w: transcription has no segments", ErrInvalidOutput)
		}
		for _, segment := range value.Segments {
			if segment.Text == "" || segment.SourceRef == "" || segment.EndSeconds < segment.StartSeconds {
				return nil, fmt.Errorf("%w: invalid transcription segment", ErrInvalidOutput)
			}
		}
	case TaskSharedMemoryRetrieval:
		var value KnowledgeResult
		if err := json.Unmarshal(raw, &value); err != nil || len(value.Entries) == 0 {
			return nil, fmt.Errorf("%w: knowledge result has no entries", ErrInvalidOutput)
		}
		for _, entry := range value.Entries {
			if entry.ReferenceID == "" || entry.Source == "" || entry.License == "" {
				return nil, fmt.Errorf("%w: knowledge citation is incomplete", ErrInvalidOutput)
			}
		}
	case TaskPhotoUnderstanding:
		var value struct {
			Description string `json:"description"`
			SourceRef   string `json:"source_ref"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || value.Description == "" || value.SourceRef == "" {
			return nil, fmt.Errorf("%w: photo result is incomplete", ErrInvalidOutput)
		}
	case TaskMaterialAssessment:
		var value MaterialAssessment
		if err := json.Unmarshal(raw, &value); err != nil || len(value.CoveredItems)+len(value.Gaps) == 0 {
			return nil, fmt.Errorf("%w: material assessment is empty", ErrInvalidOutput)
		}
		for _, item := range value.CoveredItems {
			if strings.TrimSpace(item.PlanItemID) == "" || len(item.EvidenceRefs) == 0 {
				return nil, fmt.Errorf("%w: covered item has no evidence", ErrInvalidOutput)
			}
			for _, evidence := range item.EvidenceRefs {
				if strings.TrimSpace(evidence) == "" {
					return nil, fmt.Errorf("%w: covered item evidence is empty", ErrInvalidOutput)
				}
			}
		}
		for _, gap := range value.Gaps {
			if strings.TrimSpace(gap.PlanItemID) == "" || strings.TrimSpace(gap.Reason) == "" {
				return nil, fmt.Errorf("%w: material gap is incomplete", ErrInvalidOutput)
			}
		}
	case TaskFollowupPlan:
		var value FollowupPlan
		if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value.Summary) == "" || len(value.Questions) == 0 {
			return nil, fmt.Errorf("%w: follow-up plan is empty", ErrInvalidOutput)
		}
		for _, question := range value.Questions {
			if strings.TrimSpace(question.Question) == "" {
				return nil, fmt.Errorf("%w: follow-up question is empty", ErrInvalidOutput)
			}
		}
	case TaskTimelineBuilder:
		var value struct {
			Memories []json.RawMessage `json:"memories"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || len(value.Memories) == 0 {
			return nil, fmt.Errorf("%w: timeline is empty", ErrInvalidOutput)
		}
	case TaskChapterPlanner:
		var value struct {
			Title    string            `json:"title"`
			Chapters []json.RawMessage `json:"chapters"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || value.Title == "" || len(value.Chapters) == 0 {
			return nil, fmt.Errorf("%w: chapter plan is empty", ErrInvalidOutput)
		}
	case TaskChapterWriter:
		var value struct {
			Title    string            `json:"title"`
			Chapters []json.RawMessage `json:"chapters"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || value.Title == "" || len(value.Chapters) == 0 {
			return nil, fmt.Errorf("%w: manuscript is empty", ErrInvalidOutput)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported task %q", ErrInvalidOutput, task)
	}

	buffer := bytes.NewBuffer(make([]byte, 0, len(raw)))
	if err := json.Compact(buffer, raw); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
