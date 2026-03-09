package tui

import (
	"fmt"
	"time"

	"codesnppet.dev/ifmproxy/network"
)

func RenderTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	ago := time.Since(t).Truncate(time.Second)
	return fmt.Sprintf("%s ago", ago)
}

const DOT = "●"

func RenderClient(client *network.Client) string {
	return fmt.Sprintf(
		"%s %s %s %s\n",
		boldStyle.Render(client.Addr.String()),
		subtleStyle.Render(fmt.Sprintf("rx:%d", client.Stats.Received)),
		subtleStyle.Render(fmt.Sprintf("tx:%d", client.Stats.Sent)),
		subtleStyle.Render(RenderTimeAgo(client.Stats.LastPacketAt)),
	)
}

func RenderStatus(status network.Status) string {
	switch status {
	case network.STATUS_STOPPED:
		return redStyle.Render(DOT + " Stopped")
	case network.STATUS_WAITING:
		return yellowStyle.Render(DOT + " Waiting")
	case network.STATUS_GOOD:
		return greenStyle.Render(DOT + " Connected")
	}
	return ""
}
