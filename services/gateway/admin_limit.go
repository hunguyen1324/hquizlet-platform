package main

import (
	"sync"
	"time"
)

type adminWindow struct {
	start         time.Time
	reads, writes int
}

var adminRequests = struct {
	sync.Mutex
	users map[int64]adminWindow
}{users: map[int64]adminWindow{}}

func allowAdminRequest(userID int64, mutation bool) bool {
	adminRequests.Lock()
	defer adminRequests.Unlock()
	now := time.Now()
	entry := adminRequests.users[userID]
	if now.Sub(entry.start) >= time.Minute {
		entry = adminWindow{start: now}
	}
	if len(adminRequests.users) > 10000 {
		for id, v := range adminRequests.users {
			if now.Sub(v.start) >= time.Minute {
				delete(adminRequests.users, id)
			}
		}
	}
	if mutation {
		entry.writes++
		if entry.writes > 20 {
			return false
		}
	} else {
		entry.reads++
		if entry.reads > 120 {
			return false
		}
	}
	adminRequests.users[userID] = entry
	return true
}
