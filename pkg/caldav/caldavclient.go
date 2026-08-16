package caldav

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	webdavcaldav "github.com/emersion/go-webdav/caldav"
	"github.com/joonas-fi/shopping-list-manager/pkg/taskmanager"
)

const productID = "-//shopping-list-manager//CalDAV VTODO client//EN"

type Client struct {
	caldav        *webdavcaldav.Client
	http          webdav.HTTPClient
	collectionURL *url.URL
}

var _ taskmanager.Manager = (*Client)(nil)

func NewClient(collectionURI string, username string, password string) (*Client, error) {
	collectionURL, err := url.Parse(collectionURI)
	if err != nil {
		return nil, fmt.Errorf("parse CalDAV collection URI: %w", err)
	}
	if collectionURL.Scheme != "http" && collectionURL.Scheme != "https" {
		return nil, fmt.Errorf("CalDAV collection URI must use http or https")
	}
	if collectionURL.Host == "" {
		return nil, fmt.Errorf("CalDAV collection URI must include a host")
	}
	if !strings.HasSuffix(collectionURL.Path, "/") {
		collectionURL.Path += "/"
	}

	httpClient := webdav.HTTPClientWithBasicAuth(nil, username, password)
	caldavClient, err := webdavcaldav.NewClient(httpClient, collectionURL.String())
	if err != nil {
		return nil, fmt.Errorf("create CalDAV client: %w", err)
	}

	return &Client{
		caldav:        caldavClient,
		http:          httpClient,
		collectionURL: collectionURL,
	}, nil
}

func (c *Client) Tasks(ctx context.Context) ([]taskmanager.Task, error) {
	objects, err := c.caldav.QueryCalendar(ctx, c.collectionURL.Path, &webdavcaldav.CalendarQuery{
		CompRequest: webdavcaldav.CalendarCompRequest{
			Name:     ical.CompCalendar,
			AllProps: true,
			AllComps: true,
		},
		CompFilter: webdavcaldav.CompFilter{
			Name:  ical.CompCalendar,
			Comps: []webdavcaldav.CompFilter{{Name: ical.CompToDo}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query CalDAV tasks: %w", err)
	}

	tasks := make([]taskmanager.Task, 0, len(objects))
	for _, object := range objects {
		component := primaryToDo(object.Data)
		if component == nil {
			continue
		}

		active, err := isActive(component)
		if err != nil {
			return nil, fmt.Errorf("parse CalDAV task %q: %w", object.Path, err)
		}
		if !active {
			continue
		}

		content, err := component.Props.Text(ical.PropSummary)
		if err != nil {
			return nil, fmt.Errorf("parse CalDAV task %q summary: %w", object.Path, err)
		}
		description, err := component.Props.Text(ical.PropDescription)
		if err != nil {
			return nil, fmt.Errorf("parse CalDAV task %q description: %w", object.Path, err)
		}

		tasks = append(tasks, taskmanager.Task{
			ID:          object.Path,
			Title:       content,
			Description: description,
		})
	}

	return tasks, nil
}

func (c *Client) CreateTask(ctx context.Context, task taskmanager.Task) error {
	uid, err := newUID()
	if err != nil {
		return fmt.Errorf("create CalDAV task UID: %w", err)
	}
	now := time.Now().UTC()

	toDo := ical.NewComponent(ical.CompToDo)
	toDo.Props.SetText(ical.PropUID, uid)
	toDo.Props.SetText(ical.PropSummary, task.Title)
	toDo.Props.SetText(ical.PropDescription, task.Description)
	toDo.Props.SetText(ical.PropStatus, "NEEDS-ACTION")
	toDo.Props.SetDateTime(ical.PropDateTimeStamp, now)
	toDo.Props.SetDateTime(ical.PropCreated, now)
	toDo.Props.SetDateTime(ical.PropLastModified, now)

	calendar := ical.NewCalendar()
	calendar.Props.SetText(ical.PropVersion, "2.0")
	calendar.Props.SetText(ical.PropProductID, productID)
	calendar.Children = append(calendar.Children, toDo)

	if err := c.putCalendarObject(ctx, uid+".ics", calendar, "", "*"); err != nil {
		return fmt.Errorf("create CalDAV task: %w", err)
	}

	return nil
}

func (c *Client) UpdateTask(ctx context.Context, task taskmanager.Task) error {
	object, err := c.caldav.GetCalendarObject(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("get CalDAV task: %w", err)
	}
	component := primaryToDo(object.Data)
	if component == nil {
		return fmt.Errorf("CalDAV resource %q does not contain a VTODO", task.ID)
	}

	component.Props.SetText(ical.PropSummary, task.Title)
	component.Props.SetText(ical.PropDescription, task.Description)
	now := time.Now().UTC()
	component.Props.SetDateTime(ical.PropDateTimeStamp, now)
	component.Props.SetDateTime(ical.PropLastModified, now)
	if sequence := component.Props.Get(ical.PropSequence); sequence != nil {
		value, err := sequence.Int()
		if err != nil {
			return fmt.Errorf("parse CalDAV task sequence: %w", err)
		}
		sequence.Value = strconv.Itoa(value + 1)
	}

	if err := c.putCalendarObject(ctx, task.ID, object.Data, object.ETag, ""); err != nil {
		return fmt.Errorf("update CalDAV task: %w", err)
	}

	return nil
}

func primaryToDo(calendar *ical.Calendar) *ical.Component {
	var first *ical.Component
	for _, child := range calendar.Children {
		if child.Name != ical.CompToDo {
			continue
		}
		if first == nil {
			first = child
		}
		if child.Props.Get(ical.PropRecurrenceID) == nil {
			return child
		}
	}

	return first
}

func isActive(component *ical.Component) (bool, error) {
	status, err := component.Props.Text(ical.PropStatus)
	if err != nil {
		return false, err
	}
	switch strings.ToUpper(status) {
	case "COMPLETED", "CANCELLED":
		return false, nil
	}

	if percentComplete := component.Props.Get(ical.PropPercentComplete); percentComplete != nil {
		percent, err := percentComplete.Int()
		if err != nil {
			return false, err
		}
		if percent >= 100 {
			return false, nil
		}
	}

	return true, nil
}

func newUID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	return hex.EncodeToString(random) + "@shopping-list-manager", nil
}

func (c *Client) putCalendarObject(ctx context.Context, resource string, calendar *ical.Calendar, ifMatch string, ifNoneMatch string) error {
	var body bytes.Buffer
	if err := ical.NewEncoder(&body).Encode(calendar); err != nil {
		return fmt.Errorf("encode iCalendar: %w", err)
	}

	resourceURL := c.collectionURL.ResolveReference(&url.URL{Path: resource})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, resourceURL.String(), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ical.MIMEType+"; charset=utf-8")
	if ifMatch != "" {
		req.Header.Set("If-Match", strconv.Quote(ifMatch))
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if detail := strings.TrimSpace(string(message)); detail != "" {
			return fmt.Errorf("server returned %s: %s", resp.Status, detail)
		}
		return fmt.Errorf("server returned %s", resp.Status)
	}

	return nil
}
