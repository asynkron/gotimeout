package gotimeout

import (
	"sync"
	"time"
)

type TimeoutCallback func()

type timeoutEntry struct {
	sync.Mutex
	timestamp time.Time
	callbacks []TimeoutCallback
	completed bool
}

//timeoutEntries expires after 500 milliseconds
func (te *timeoutEntry) expired() bool {
	return te.timestamp.Before(time.Now().Add(-time.Duration(500) * time.Millisecond))
}

func (te *timeoutEntry) AddCallback(callback TimeoutCallback) {
	if te.completed {
		return //TODO error? this should not happen so...
	}

	te.Lock()
	defer te.Unlock()
	te.callbacks = append(te.callbacks, callback)
}

func (te *timeoutEntry) trigger() {
	te.Lock()
	defer te.Unlock()
	te.completed = true
	for _, callback := range te.callbacks {
		callback()
	}
}

type Timeout struct {
	entries [60 * 10]*timeoutEntry //we support 10 minutes timeouts with caching, else unique instance
}

var timeout = &Timeout{}

func AfterFunc(seconds int, callback TimeoutCallback) {
	timeout.AfterFunc(seconds, callback)
}

// AfterFunc works similar to time.AfterFunc, with the difference that timers are cached based on the timeout length
// therefore Seconds are used as a "granular enough" unit for caching
// any timeout entry older than 500ms will be recreated and overwritten
// TLDR; the purpose of all this is to avoid spawning thousands of timers under heavy load
// the standard usecase would be to use a timeout for some form of request, where the timeout is a few seconds
// due to the 500ms expiration, if a timeout is setup using AfterFunc(10), this in reality means 9.5-10 seconds before timeout
func (t *Timeout) AfterFunc(seconds int, callback TimeoutCallback) {
	//no timeout, just invoke it
	if seconds == 0 {
		callback()
		return
	}

	if seconds > len(t.entries)-1 {
		//just use a unique instance
		timeout := time.Duration(seconds) * time.Second
		time.AfterFunc(timeout, callback)
		return
	}

	//fetch entry from entry array
	entry := t.entries[seconds]

	//if entry doesn't exist, or if entry has expired, recreate it
	if entry == nil || entry.expired() {
		entry = &timeoutEntry{
			timestamp: time.Now(),
		}
		//this is racy and we don't care, it's OK if it's overwritten, wasting an entry is cheaper than locking
		t.entries[seconds] = entry
		timeout := time.Duration(seconds) * time.Second
		time.AfterFunc(timeout, entry.trigger)
	}

	entry.AddCallback(callback)
}
