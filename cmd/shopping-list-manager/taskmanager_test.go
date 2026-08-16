package main

import (
	"strings"
	"testing"

	"github.com/joonas-fi/shopping-list-manager/pkg/caldav"
	"github.com/joonas-fi/shopping-list-manager/pkg/todoist"
)

func TestGetTaskManager(t *testing.T) {
	t.Run("Todoist is the default", func(t *testing.T) {
		t.Setenv("TASK_MANAGER", "")
		t.Setenv("TODOIST_TOKEN", "token")
		t.Setenv("TODOIST_PROJECT_ID", "project")

		manager, err := getTaskManager()
		if err != nil {
			t.Fatalf("getTaskManager: %v", err)
		}
		if _, ok := manager.(*todoist.Client); !ok {
			t.Fatalf("default manager is %T, want *todoist.Client", manager)
		}
	})

	t.Run("CalDAV", func(t *testing.T) {
		t.Setenv("TASK_MANAGER", "caldav")
		t.Setenv("CALDAV_URI", "https://caldav.example.test/dav/tasks/")
		t.Setenv("CALDAV_USERNAME", "user")
		t.Setenv("CALDAV_PASSWORD", "password")

		manager, err := getTaskManager()
		if err != nil {
			t.Fatalf("getTaskManager: %v", err)
		}
		if _, ok := manager.(*caldav.Client); !ok {
			t.Fatalf("selected manager is %T, want *caldav.Client", manager)
		}
	})

	t.Run("Unknown provider", func(t *testing.T) {
		t.Setenv("TASK_MANAGER", "other")

		_, err := getTaskManager()
		if err == nil || !strings.Contains(err.Error(), "unsupported TASK_MANAGER") {
			t.Fatalf("getTaskManager error = %v", err)
		}
	})
}
