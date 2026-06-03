package system

import (
	"fmt"
	"os"
	"strings"
)

type SessionKind int

const (
	SessionX11 SessionKind = iota
	SessionWayland
	SessionXWayland
	SessionUnknown
)

type Info struct {
	Kind           SessionKind
	XDGSessionType string
	Display        string
	WaylandDisplay string
}

func Detect() Info {
	xdg := os.Getenv("XDG_SESSION_TYPE")
	display := os.Getenv("DISPLAY")
	wayland := os.Getenv("WAYLAND_DISPLAY")

	var kind SessionKind
	switch strings.ToLower(xdg) {
	case "x11":
		kind = SessionX11
	case "wayland":
		if display != "" {
			kind = SessionXWayland
		} else {
			kind = SessionWayland
		}
	default:
		switch {
		case display != "" && wayland == "":
			kind = SessionX11
		case display != "" && wayland != "":
			kind = SessionXWayland
		case wayland != "":
			kind = SessionWayland
		default:
			kind = SessionUnknown
		}
	}

	return Info{
		Kind:           kind,
		XDGSessionType: xdg,
		Display:        display,
		WaylandDisplay: wayland,
	}
}

func (i Info) IsX11() bool {
	return i.Kind == SessionX11
}

func (i Info) KindLabel() string {
	switch i.Kind {
	case SessionX11:
		return "X11"
	case SessionWayland:
		return "Wayland"
	case SessionXWayland:
		return "XWayland"
	default:
		return "unknown"
	}
}

// Warning returns a non-empty string when the session may not work with x11droid.
func (i Info) Warning() string {
	switch i.Kind {
	case SessionWayland:
		return fmt.Sprintf("session is Wayland (%s) — display forwarding requires X11; cage cannot connect to a Wayland compositor", i.WaylandDisplay)
	case SessionXWayland:
		return fmt.Sprintf("running under XWayland ($DISPLAY=%s, $WAYLAND_DISPLAY=%s) — may work but is untested", i.Display, i.WaylandDisplay)
	case SessionUnknown:
		if i.Display == "" {
			return "$DISPLAY is not set — cage has no X11 server to connect to"
		}
		return fmt.Sprintf("$XDG_SESSION_TYPE unset, assuming X11 ($DISPLAY=%s)", i.Display)
	}
	return ""
}
