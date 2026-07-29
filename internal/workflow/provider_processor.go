package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type JobSubmitter interface {
	Submit(context.Context, providerjobs.SubmitInput) (providerjobs.Job, error)
}

type ProviderInputBuilder func(context.Context, NodePayload) (json.RawMessage, []string, error)

type ProviderProcessor struct {
	submitter    JobSubmitter
	providerKind providers.Kind
	taskType     providers.TaskType
	buildInput   ProviderInputBuilder
	callbackBase string
}

func NewProviderProcessor(
	submitter JobSubmitter,
	providerKind providers.Kind,
	taskType providers.TaskType,
	buildInput ProviderInputBuilder,
	callbackBaseURL string,
) *ProviderProcessor {
	return &ProviderProcessor{
		submitter: submitter, providerKind: providerKind, taskType: taskType,
		buildInput: buildInput, callbackBase: strings.TrimRight(callbackBaseURL, "/"),
	}
}

func (p *ProviderProcessor) Process(ctx context.Context, payload NodePayload) (ProcessResult, error) {
	input, resources, err := p.buildInput(ctx, payload)
	if err != nil {
		return ProcessResult{}, err
	}
	job, err := p.submitter.Submit(ctx, providerjobs.SubmitInput{
		ProjectID: payload.ProjectID, WorkflowRunID: payload.RunID, WorkflowNode: string(payload.Node),
		ProviderKind: p.providerKind, TaskType: p.taskType, Input: input, ResourceURLs: resources,
		CallbackBaseURL: p.callbackBase, Deadline: time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		return ProcessResult{}, err
	}
	return Waiting(job.ID), nil
}
