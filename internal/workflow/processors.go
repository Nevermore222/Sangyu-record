package workflow

import (
	"context"
	"encoding/json"
)

type DeterministicProcessor struct {
	Node NodeName
}

func (p DeterministicProcessor) Process(_ context.Context, payload NodePayload) (json.RawMessage, error) {
	var output any
	switch p.Node {
	case NodeTranscribe:
		output = map[string]any{
			"segments": []map[string]any{{"start_seconds": 12, "end_seconds": 20, "text": "1978年，我进入了当地纺织厂。", "source": "audio-fixture#12-20"}},
		}
	case NodeUnderstandPhoto:
		output = map[string]any{"description": "候选描述：一张工作时期的集体照片", "source": "photo-fixture", "confidence": "inferred"}
	case NodeBuildMemory:
		output = map[string]any{"memories": []map[string]any{{"event": "进入当地纺织厂工作", "year": 1978, "evidence_refs": []string{"audio-fixture#12-20"}}}}
	case NodePlanBook:
		output = map[string]any{"title": "岁月留声", "chapters": []map[string]any{{"title": "纺织厂的日子", "target_words": 1200}}}
	case NodeWriteBook:
		output = map[string]any{
			"title": "岁月留声",
			"chapters": []map[string]any{{"title": "纺织厂的日子", "paragraphs": []map[string]any{{
				"text": "1978年，她进入了当地纺织厂。", "evidence_refs": []string{"audio-fixture#12-20"},
			}}}},
		}
	case NodeRenderPDF:
		return nil, ErrRendererUnavailable
	default:
		return nil, ErrProcessorMissing
	}
	encoded, err := json.Marshal(map[string]any{"project_id": payload.ProjectID, "output": output})
	return encoded, err
}

func DeterministicProcessors() map[NodeName]Processor {
	processors := make(map[NodeName]Processor, len(NodeSequence))
	for _, node := range NodeSequence {
		processors[node] = DeterministicProcessor{Node: node}
	}
	return processors
}
