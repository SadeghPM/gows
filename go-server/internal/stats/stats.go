package stats

import (
	"sync/atomic"
	"time"
)

type ServerStats struct {
	StartedAt              time.Time
	WSAttempts             atomic.Uint64
	WSAccepted             atomic.Uint64
	WSRejected             atomic.Uint64
	BroadcastRequests      atomic.Uint64
	BroadcastUnauthorized  atomic.Uint64
	BroadcastSent          atomic.Uint64
	BroadcastFailed        atomic.Uint64
	ValidationRequests     atomic.Uint64
	ValidationFailures     atomic.Uint64
	AdminUnauthorizedViews atomic.Uint64
}

var Server ServerStats

func Init() {
	Server.StartedAt = time.Now()
}
