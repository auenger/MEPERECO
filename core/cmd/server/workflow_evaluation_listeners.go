package main

import (
	"github.com/auenger/MEPERECO/core/internal/events"
	"github.com/auenger/MEPERECO/core/internal/handler"
	"github.com/auenger/MEPERECO/core/internal/service"
	"github.com/auenger/MEPERECO/core/internal/util"
	"github.com/auenger/MEPERECO/core/pkg/protocol"
)

func registerWorkflowEvaluationListeners(bus *events.Bus) <-chan service.WorkflowEvaluationTarget {
	queue := make(chan service.WorkflowEvaluationTarget, 256)
	handlerFn := func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		var id string
		if task, ok := payload["task"].(handler.TaskResponse); ok && task.WorkflowInstanceID != nil {
			id = *task.WorkflowInstanceID
		}
		if id == "" {
			switch v := payload["workflow_instance_id"].(type) {
			case string:
				id = v
			case *string:
				if v != nil {
					id = *v
				}
			}
		}
		if id == "" {
			return
		}
		ws, err := util.ParseUUID(e.WorkspaceID)
		if err != nil {
			return
		}
		instance, err := util.ParseUUID(id)
		if err != nil {
			return
		}
		// Never block the synchronous event bus. Reconciliation recovers dropped events.
		select {
		case queue <- service.WorkflowEvaluationTarget{WorkspaceID: ws, InstanceID: instance}:
		default:
		}
	}
	for _, event := range []string{protocol.EventTaskCreated, protocol.EventTaskUpdated, protocol.EventTaskDeleted} {
		bus.Subscribe(event, handlerFn)
	}
	return queue
}
