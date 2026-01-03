// Todoist client
package todoist

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/function61/gokit/net/http/ezhttp"
)

// https://developer.todoist.com/rest/v2/

type Project struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Task struct {
	ID          string    `json:"id"`
	Order       int       `json:"order,omitempty"` // order within this project. on creation need omitempty to not set 0 (= which would be first on list)
	Content     string    `json:"content"`
	Description string    `json:"description"`
	Completed   bool      `json:"is_completed"`
	Created     time.Time `json:"created_at"`
	URL         string    `json:"url"`
	Due         *DueSpec  `json:"due"` // only present for ones that have due date

	ProjectID string `json:"project_id"`
}

// returns 0 if no due date
// NOTE: returned value can be negative
func (t Task) OverdueAmount(now time.Time) time.Duration {
	if t.Due != nil {
		return t.Due.Overdue(now)
	} else {
		return time.Duration(0)
	}
}

type DueSpec struct {
	Recurring bool          `json:"is_recurring"`
	Date      JSONPlainDate `json:"date"` // looks like: 2021-01-15
}

// NOTE: returned value can be negative
func (d DueSpec) Overdue(now time.Time) time.Duration {
	return now.Sub(d.Date.Time)
}

func NewClient(token string) *Client {
	return &Client{token}
}

type Client struct {
	token string
}

func (t *Client) Project(ctx context.Context, id int64) (*Project, error) {
	project := &Project{}

	if _, err := ezhttp.Get(ctx, fmt.Sprintf("https://api.todoist.com/rest/v2/projects/%d", id),
		ezhttp.AuthBearer(t.token),
		ezhttp.RespondsJSONAllowUnknownFields(project),
	); err != nil {
		return nil, fmt.Errorf("Project: %w", err)
	}

	return project, nil
}

func (t *Client) TasksByProject(ctx context.Context, id int64, now time.Time) ([]Task, error) {
	tasks := []Task{}

	if _, err := ezhttp.Get(ctx, fmt.Sprintf("https://api.todoist.com/rest/v2/tasks?project_id=%d", id),
		ezhttp.AuthBearer(t.token),
		ezhttp.RespondsJSONAllowUnknownFields(&tasks),
	); err != nil {
		return nil, fmt.Errorf("TasksByProject: %w", err)
	}

	// REST API results have no ordering, so we have to sort them.
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Order < tasks[j].Order
	})

	return tasks, nil
}

func (t *Client) CreateTask(ctx context.Context, task Task) error {
	if _, err := ezhttp.Post(ctx, "https://api.todoist.com/rest/v2/tasks",
		ezhttp.AuthBearer(t.token),
		ezhttp.SendJSON(task),
	); err != nil {
		return fmt.Errorf("CreateTask: %w", err)
	}

	return nil
}

func (t *Client) UpdateTask(ctx context.Context, task Task) error {
	if _, err := ezhttp.Post(ctx, fmt.Sprintf("https://api.todoist.com/rest/v2/tasks/%s", task.ID),
		ezhttp.AuthBearer(t.token),
		ezhttp.SendJSON(task),
	); err != nil {
		return fmt.Errorf("UpdateTask: %w", err)
	}

	return nil
}
