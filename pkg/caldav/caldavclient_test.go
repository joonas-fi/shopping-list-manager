package caldav

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersion/go-ical"
	"github.com/joonas-fi/shopping-list-manager/pkg/taskmanager"
)

const testCollectionPath = "/dav/tasks/"

func TestTasksReturnsOnlyActiveVTODOs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertBasicAuth(t, r)
		if r.Method != "REPORT" || r.URL.Path != testCollectionPath {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read REPORT: %v", err)
		}
		if !strings.Contains(string(body), `name="VTODO"`) {
			t.Errorf("REPORT does not filter VTODOs: %s", body)
		}

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, multiStatus(
			calendarResponse("active.ics", vtodo("active", "Milk", "details", "NEEDS-ACTION", ""), "active-tag"),
			calendarResponse("completed.ics", vtodo("completed", "Bread", "", "COMPLETED", ""), "completed-tag"),
			calendarResponse("percent.ics", vtodo("percent", "Eggs", "", "IN-PROCESS", "PERCENT-COMPLETE:100\r\n"), "percent-tag"),
		))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	tasks, err := client.Tasks(context.Background())
	if err != nil {
		t.Fatalf("Tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %#v", len(tasks), tasks)
	}
	if tasks[0].ID != testCollectionPath+"active.ics" || tasks[0].Title != "Milk" || tasks[0].Description != "details" {
		t.Fatalf("unexpected task: %#v", tasks[0])
	}
}

func TestCreateTaskWritesVTODO(t *testing.T) {
	requestSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen = true
		assertBasicAuth(t, r)
		if r.Method != http.MethodPut || !strings.HasPrefix(r.URL.Path, testCollectionPath) || !strings.HasSuffix(r.URL.Path, ".ics") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("If-None-Match"); got != "*" {
			t.Errorf("If-None-Match = %q, want *", got)
		}

		calendar, err := ical.NewDecoder(r.Body).Decode()
		if err != nil {
			t.Errorf("decode calendar: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		component := primaryToDo(calendar)
		if component == nil {
			t.Error("created calendar has no VTODO")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assertTextProp(t, component, ical.PropSummary, "Coffee")
		assertTextProp(t, component, ical.PropDescription, "[Details](https://example.test/item)")
		assertTextProp(t, component, ical.PropStatus, "NEEDS-ACTION")
		if component.Props.Get(ical.PropUID) == nil || component.Props.Get(ical.PropDateTimeStamp) == nil {
			t.Error("created VTODO lacks UID or DTSTAMP")
		}
		if component.Props.Get(ical.PropPriority) != nil {
			t.Error("Todoist ordering must not be mapped to VTODO priority")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	err := client.CreateTask(context.Background(), taskmanager.Task{
		Title:       "Coffee",
		Description: "[Details](https://example.test/item)",
		Order:       12300,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if !requestSeen {
		t.Fatal("CalDAV PUT was not made")
	}
}

func TestUpdateTaskPreservesCalendarData(t *testing.T) {
	const existing = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nBEGIN:VTODO\r\nUID:item-1\r\nDTSTAMP:20200101T000000Z\r\nSUMMARY:Old name\r\nDESCRIPTION:details\r\nSEQUENCE:4\r\nX-CLIENT-PROPERTY:preserve me\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	putSeen := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/calendar")
			w.Header().Set("ETag", `"old-tag"`)
			_, _ = io.WriteString(w, existing)
		case http.MethodPut:
			putSeen = true
			if got := r.Header.Get("If-Match"); got != `"old-tag"` {
				t.Errorf("If-Match = %q, want quoted old-tag", got)
			}
			calendar, err := ical.NewDecoder(r.Body).Decode()
			if err != nil {
				t.Errorf("decode updated calendar: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			component := primaryToDo(calendar)
			if component == nil {
				t.Error("updated calendar has no VTODO")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			assertTextProp(t, component, ical.PropSummary, "New name")
			assertTextProp(t, component, "X-CLIENT-PROPERTY", "preserve me")
			if sequence := component.Props.Get(ical.PropSequence); sequence == nil || sequence.Value != "5" {
				t.Errorf("SEQUENCE = %#v, want 5", sequence)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	err := client.UpdateTask(context.Background(), taskmanager.Task{
		ID:          testCollectionPath + "item.ics",
		Title:       "New name",
		Description: "details",
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if !putSeen {
		t.Fatal("CalDAV update PUT was not made")
	}
}

func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	client, err := NewClient(serverURL+testCollectionPath, "user", "password")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func assertBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	username, password, ok := r.BasicAuth()
	if !ok || username != "user" || password != "password" {
		t.Errorf("unexpected Basic auth: %q %q, present=%v", username, password, ok)
	}
}

func assertTextProp(t *testing.T, component *ical.Component, name string, want string) {
	t.Helper()
	got, err := component.Props.Text(name)
	if err != nil {
		t.Errorf("read %s: %v", name, err)
		return
	}
	if got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func multiStatus(responses ...string) string {
	return `<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">` + strings.Join(responses, "") + `</d:multistatus>`
}

func calendarResponse(name string, calendar string, etag string) string {
	return fmt.Sprintf(`<d:response><d:href>%s%s</d:href><d:propstat><d:prop><c:calendar-data>%s</c:calendar-data><d:getetag>&quot;%s&quot;</d:getetag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`,
		testCollectionPath, name, html.EscapeString(calendar), etag)
}

func vtodo(uid string, summary string, description string, status string, extra string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\nBEGIN:VTODO\r\nUID:" + uid + "\r\nDTSTAMP:20200101T000000Z\r\nSUMMARY:" + summary + "\r\nDESCRIPTION:" + description + "\r\nSTATUS:" + status + "\r\n" + extra + "END:VTODO\r\nEND:VCALENDAR\r\n"
}
