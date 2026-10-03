package main

import (
	"testing"

	"github.com/joonas-fi/shopping-list-manager/pkg/caldav"
)

func TestGetTaskManager(t *testing.T) {
	t.Setenv("CALDAV_URI", "https://caldav.example.test/dav/tasks/")
	t.Setenv("CALDAV_USERNAME", "user")
	t.Setenv("CALDAV_PASSWORD", "password")

	manager, err := getTaskManager()
	if err != nil {
		t.Fatalf("getTaskManager: %v", err)
	}
	if _, ok := manager.(*caldav.Client); !ok {
		t.Fatalf("manager is %T, want *caldav.Client", manager)
	}
}
