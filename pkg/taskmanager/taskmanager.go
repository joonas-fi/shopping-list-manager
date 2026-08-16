package taskmanager

import "context"

type Task struct {
	ID          string
	Title       string
	Description string
	Order       int
}

type Manager interface {
	Tasks(ctx context.Context) ([]Task, error)
	CreateTask(ctx context.Context, task Task) error
	UpdateTask(ctx context.Context, task Task) error
}
