// Todoist client
package todoist

import (
	"context"
	"fmt"
	"sort"

	"github.com/function61/gokit/net/http/ezhttp"
	"github.com/joonas-fi/shopping-list-manager/pkg/taskmanager"
)

// https://developer.todoist.com/api/v1/

// type Project struct {
// 	Name string `json:"name"`
// 	URL  string `json:"url"`
// }

type task struct {
	ID          string `json:"id"`
	Order       int    `json:"order,omitempty"`       // (ONLY USED WHEN CREATING - GENIUS DESIGN!!!!) order within this project. on creation need omitempty to not set 0 (= which would be first on list).
	ChildOrder  int    `json:"child_order,omitempty"` // (ONLY USED WHEN LISTING  - GENIUS DESIGN!!!!) order within this project (named "child" even though the perspective is this task, more apt would've been "order_in_parent").
	Content     string `json:"content"`
	Description string `json:"description"`
	CompletedAt bool   `json:"completed_at,omitempty"`
	ProjectID   string `json:"project_id"`
}

type paginated[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor"`
}

func NewClient(token string, projectID string) *Client {
	return &Client{token, projectID}
}

type Client struct {
	token     string
	projectID string
}

var _ taskmanager.Manager = (*Client)(nil)

// func (t *Client) Project(ctx context.Context, id int64) (*Project, error) {
// 	project := &Project{}

// 	if _, err := ezhttp.Get(ctx, fmt.Sprintf("https://api.todoist.com/rest/v2/projects/%d", id),
// 		ezhttp.AuthBearer(t.token),
// 		ezhttp.RespondsJSONAllowUnknownFields(project),
// 	); err != nil {
// 		return nil, fmt.Errorf("Project: %w", err)
// 	}

// 	return project, nil
// }

func (t *Client) Tasks(ctx context.Context) ([]taskmanager.Task, error) {
	tasksPaginated := paginated[task]{}

	if _, err := ezhttp.Get(ctx, fmt.Sprintf("https://api.todoist.com/api/v1/tasks?project_id=%s&limit=200", t.projectID),
		ezhttp.AuthBearer(t.token),
		ezhttp.RespondsJSONAllowUnknownFields(&tasksPaginated),
	); err != nil {
		return nil, fmt.Errorf("Tasks: %w", err)
	}

	if cursor := tasksPaginated.NextCursor; cursor != nil {
		return nil, fmt.Errorf("Tasks: got paginated results which we don't yet support: %s", *cursor)
	}

	tasks := tasksPaginated.Results // unfuck

	// REST API results have no ordering, so we have to sort them.
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ChildOrder < tasks[j].ChildOrder
	})

	result := make([]taskmanager.Task, len(tasks))
	for i, task := range tasks {
		result[i] = taskmanager.Task{
			ID:          task.ID,
			Title:       task.Content,
			Description: task.Description,
		}
	}

	return result, nil
}

func (t *Client) CreateTask(ctx context.Context, newTask taskmanager.Task) error {
	if _, err := ezhttp.Post(ctx, "https://api.todoist.com/api/v1/tasks",
		ezhttp.AuthBearer(t.token),
		ezhttp.SendJSON(task{
			Content:     newTask.Title,
			Description: newTask.Description,
			ProjectID:   t.projectID,
			Order:       newTask.Order,
		}),
	); err != nil {
		return fmt.Errorf("CreateTask: %w", err)
	}

	return nil
}

func (t *Client) UpdateTask(ctx context.Context, updatedTask taskmanager.Task) error {
	// POST to update task, genius 👍
	if _, err := ezhttp.Post(ctx, fmt.Sprintf("https://api.todoist.com/api/v1/tasks/%s", updatedTask.ID),
		ezhttp.AuthBearer(t.token),
		ezhttp.SendJSON(task{
			Content:     updatedTask.Title,
			Description: updatedTask.Description,
		}),
	); err != nil {
		return fmt.Errorf("UpdateTask: %w", err)
	}

	return nil
}
