# Local patches

## Encode CalDAV `PropFilter.IsNotDefined`

Based on upstream `github.com/emersion/go-webdav` `v0.7.0`.

This version exposes `caldav.PropFilter.IsNotDefined`, but the calendar-query
encoder did not serialize it. The local patch in `caldav/client.go` emits the
required `CALDAV:is-not-defined` element. Remove this override after the
upstream fix is released.
