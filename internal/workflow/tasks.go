package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const TaskWorkflowNode = "workflow:node"

type AsynqEnqueuer struct {
	client *asynq.Client
}

func NewAsynqEnqueuer(client *asynq.Client) *AsynqEnqueuer {
	return &AsynqEnqueuer{client: client}
}

func (q *AsynqEnqueuer) Enqueue(ctx context.Context, payload NodePayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := asynq.NewTask(TaskWorkflowNode, encoded)
	_, err = q.client.EnqueueContext(ctx, task,
		asynq.TaskID(fmt.Sprintf("%s:%s", payload.RunID, payload.Node)),
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
	)
	return err
}

type AsynqHandler struct {
	worker *Worker
}

func NewAsynqHandler(worker *Worker) *AsynqHandler {
	return &AsynqHandler{worker: worker}
}

func (h *AsynqHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload NodePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode workflow payload: %w", err)
	}
	return h.worker.Process(ctx, payload)
}
