package sdlc

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrSecurityGateFailed = errors.New("security gate failed")
)

type Stage string

const (
	StageProduct   Stage = "product"
	StageDeveloper Stage = "developer"
	StageSecurity  Stage = "security"
)

type SDLCRequest struct {
	WorkflowID  string
	ProjectID   string
	Goal        string
	Constraints []string
}

type WorkflowRun struct {
	WorkflowID string
	ProjectID  string
	Status     string
	Attempt    int
	Stage      Stage
	LastError  string
}

type A2ATask struct {
	WorkflowID string
	FromAgent  string
	ToAgent    string
	TaskType   string
	Payload    string
}

type WorkflowStore interface {
	CreateRun(ctx context.Context, run WorkflowRun) error
	UpdateRun(ctx context.Context, run WorkflowRun) error
	FindRun(ctx context.Context, workflowID string) (WorkflowRun, error)
}

type WorkflowEngine interface {
	ExecuteProduct(ctx context.Context, req SDLCRequest) error
	ExecuteDeveloper(ctx context.Context, req SDLCRequest) error
	ExecuteSecurity(ctx context.Context, req SDLCRequest) error
}

type A2ACommunicator interface {
	SendTask(ctx context.Context, task A2ATask) error
}

type noopCommunicator struct{}

func (noopCommunicator) SendTask(context.Context, A2ATask) error { return nil }

type Coordinator struct {
	store        WorkflowStore
	engine       WorkflowEngine
	communicator A2ACommunicator
	maxRetries   int
}

func NewCoordinator(store WorkflowStore, engine WorkflowEngine, maxRetries int) *Coordinator {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Coordinator{store: store, engine: engine, communicator: noopCommunicator{}, maxRetries: maxRetries}
}

func (c *Coordinator) WithA2ACommunicator(communicator A2ACommunicator) *Coordinator {
	if communicator == nil {
		c.communicator = noopCommunicator{}
		return c
	}
	c.communicator = communicator
	return c
}

func (c *Coordinator) Run(ctx context.Context, req SDLCRequest) error {
	run := WorkflowRun{WorkflowID: req.WorkflowID, ProjectID: req.ProjectID, Status: "running", Attempt: 0, Stage: StageProduct}
	if err := c.store.CreateRun(ctx, run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}

	if err := c.executeProductAndNotifyDeveloper(ctx, req); err != nil {
		run.Status = "failed"
		run.LastError = err.Error()
		_ = c.store.UpdateRun(ctx, run)
		return err
	}

	run.Stage = StageDeveloper
	if err := c.store.UpdateRun(ctx, run); err != nil {
		return fmt.Errorf("persist developer stage: %w", err)
	}
	if err := c.executeDeveloperWithClarification(ctx, req); err != nil {
		run.Status = "failed"
		run.LastError = err.Error()
		_ = c.store.UpdateRun(ctx, run)
		return err
	}
	if err := c.communicator.SendTask(ctx, A2ATask{WorkflowID: req.WorkflowID, FromAgent: "developer", ToAgent: "security", TaskType: "changeset_ready", Payload: "developer changes ready for scanning"}); err != nil {
		run.Status = "failed"
		run.LastError = err.Error()
		_ = c.store.UpdateRun(ctx, run)
		return fmt.Errorf("a2a developer->security: %w", err)
	}

	return c.runSecurityLoop(ctx, req, run)
}

func (c *Coordinator) executeProductAndNotifyDeveloper(ctx context.Context, req SDLCRequest) error {
	if err := c.engine.ExecuteProduct(ctx, req); err != nil {
		return fmt.Errorf("product stage: %w", err)
	}
	if err := c.communicator.SendTask(ctx, A2ATask{WorkflowID: req.WorkflowID, FromAgent: "product", ToAgent: "developer", TaskType: "prd_ready", Payload: "product artifact ready"}); err != nil {
		return fmt.Errorf("a2a product->developer: %w", err)
	}
	return nil
}

func (c *Coordinator) executeDeveloperWithClarification(ctx context.Context, req SDLCRequest) error {
	if err := c.engine.ExecuteDeveloper(ctx, req); err == nil {
		return nil
	}

	if err := c.communicator.SendTask(ctx, A2ATask{WorkflowID: req.WorkflowID, FromAgent: "developer", ToAgent: "product", TaskType: "requirements_clarification_required", Payload: "developer needs requirement clarity"}); err != nil {
		return fmt.Errorf("a2a developer->product clarification: %w", err)
	}
	if err := c.executeProductAndNotifyDeveloper(ctx, req); err != nil {
		return fmt.Errorf("requirements clarification loop: %w", err)
	}
	if err := c.engine.ExecuteDeveloper(ctx, req); err != nil {
		return fmt.Errorf("developer stage: %w", err)
	}
	return nil
}

func (c *Coordinator) runSecurityLoop(ctx context.Context, req SDLCRequest, run WorkflowRun) error {
	run.Stage = StageSecurity
	if err := c.store.UpdateRun(ctx, run); err != nil {
		return fmt.Errorf("persist security stage: %w", err)
	}

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		run.Attempt = attempt
		if err := c.engine.ExecuteSecurity(ctx, req); err == nil {
			run.Status = "completed"
			run.LastError = ""
			if persistErr := c.store.UpdateRun(ctx, run); persistErr != nil {
				return fmt.Errorf("persist success: %w", persistErr)
			}
			return nil
		}

		if attempt == c.maxRetries {
			run.Status = "failed"
			run.LastError = ErrSecurityGateFailed.Error()
			_ = c.store.UpdateRun(ctx, run)
			return ErrSecurityGateFailed
		}

		if err := c.communicator.SendTask(ctx, A2ATask{WorkflowID: req.WorkflowID, FromAgent: "security", ToAgent: "developer", TaskType: "remediation_required", Payload: "security findings require remediation"}); err != nil {
			run.Status = "failed"
			run.LastError = err.Error()
			_ = c.store.UpdateRun(ctx, run)
			return fmt.Errorf("a2a security->developer: %w", err)
		}

		if err := c.executeDeveloperWithClarification(ctx, req); err != nil {
			run.Status = "failed"
			run.LastError = err.Error()
			_ = c.store.UpdateRun(ctx, run)
			return fmt.Errorf("developer remediation: %w", err)
		}

		if err := c.communicator.SendTask(ctx, A2ATask{WorkflowID: req.WorkflowID, FromAgent: "developer", ToAgent: "security", TaskType: "remediation_ready", Payload: "remediation changes ready for re-scan"}); err != nil {
			run.Status = "failed"
			run.LastError = err.Error()
			_ = c.store.UpdateRun(ctx, run)
			return fmt.Errorf("a2a developer->security remediation: %w", err)
		}
	}
	return ErrSecurityGateFailed
}
