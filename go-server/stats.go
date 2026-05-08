package main

import (
	"sync/atomic"
	"time"
)

var stats ServerStats

type ServerStats struct {
	startedAt              time.Time
	wsAttempts             atomic.Uint64
	wsAccepted             atomic.Uint64
	wsRejected             atomic.Uint64
	broadcastRequests      atomic.Uint64
	broadcastUnauthorized  atomic.Uint64
	broadcastSent          atomic.Uint64
	broadcastFailed        atomic.Uint64
	validationRequests     atomic.Uint64
	validationFailures     atomic.Uint64
	adminUnauthorizedViews atomic.Uint64
}
