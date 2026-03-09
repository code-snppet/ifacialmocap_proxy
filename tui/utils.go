package tui

import (
	"fmt"
	"time"

	"codesnppet.dev/ifmproxy/relay"
)

func RenderTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	ago := time.Since(t).Truncate(time.Second)
	return fmt.Sprintf("%s ago", ago)
}

const DOT = "●"

func RenderClient(client *relay.Client) string {
	return fmt.Sprintf(
		"%s %s %s %s\n",
		boldStyle.Render(client.Addr.String()),
		subtleStyle.Render(fmt.Sprintf("rx:%d", client.Stats.Received)),
		subtleStyle.Render(fmt.Sprintf("tx:%d", client.Stats.Sent)),
		subtleStyle.Render(RenderTimeAgo(client.Stats.LastPacketAt)),
	)
}

func RenderStatus(status relay.Status) string {
	switch status {
	case relay.STATUS_STOPPED:
		return redStyle.Render(DOT + " Stopped")
	case relay.STATUS_WAITING:
		return yellowStyle.Render(DOT + " Waiting")
	case relay.STATUS_GOOD:
		return greenStyle.Render(DOT + " Connected")
	}
	return ""
}
