module github.com/joonas-fi/shopping-list-manager

go 1.23.1

require (
	github.com/emersion/go-ical v0.0.0-20240127095438-fc1c9d8fb2b6
	github.com/emersion/go-webdav v0.7.0
	github.com/function61/gokit v0.0.0-20250704123853-66cf16f69a87
	github.com/joonas-fi/home-audio v0.0.0-20250201142352-c32d8a7f5a47
	github.com/samber/lo v1.47.0
	github.com/spf13/cobra v1.6.1
)

require (
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/lmittmann/tint v1.0.5 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pkg/xattr v0.4.4 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/teambition/rrule-go v1.8.2 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.16.0 // indirect
)

// Patched locally until https://github.com/emersion/go-webdav issues a fix for
// caldav.PropFilter.IsNotDefined not being encoded in calendar-query requests.
replace github.com/emersion/go-webdav => ./third_party/go-webdav
