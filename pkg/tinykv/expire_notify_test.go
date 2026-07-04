package tinykv

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestExpireFuncNotifiesEveryDeletedEntry guards against a silent-drop
// regression in expireFunc: every entry removed from the store on expiration
// MUST fire onExpire. The bug occurred when an expiry batch mixed genuinely
// expired keys with keys that had been re-Put (a stale timeout still queued in
// the heap); the re-validation pass could delete survivors from the store yet
// drop them from the notification set, leaving an expired instance running with
// no session (an orphan).
func TestExpireFuncNotifiesEveryDeletedEntry(t *testing.T) {
	for trial := 0; trial < 200; trial++ {
		var mu sync.Mutex
		notified := map[string]bool{}
		notifiedCount := func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(notified)
		}
		kv := New[int](time.Hour, func(k string, _ int) {
			mu.Lock()
			notified[k] = true
			mu.Unlock()
		}).(*store[int])

		const survivors = 30
		const reputs = 30
		for i := 0; i < survivors; i++ {
			_ = kv.Put(fmt.Sprintf("s%02d", i), i, -time.Second)
		}
		for i := 0; i < reputs; i++ {
			k := fmt.Sprintf("r%02d", i)
			_ = kv.Put(k, i, -time.Second) // stale timeout left in the heap
			_ = kv.Put(k, i, time.Hour)    // current entry, not expired
		}

		// expireFunc removes expired entries from the store synchronously and
		// dispatches onExpire asynchronously. Drain the heap, then wait for the
		// notifications to catch up rather than sleeping a fixed amount: poll
		// until every survivor has been notified, up to a generous deadline. A
		// dropped notification never arrives, so the loop hits the deadline and
		// the assertions below fail deterministically (not on scheduling jitter).
		for i := 0; i < survivors+2*reputs+5; i++ {
			kv.expireFunc()
		}
		deadline := time.Now().Add(2 * time.Second)
		for notifiedCount() < survivors && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}

		mu.Lock()
		for i := 0; i < survivors; i++ {
			k := fmt.Sprintf("s%02d", i)
			kv.mx.Lock()
			_, inStore := kv.kv[k]
			kv.mx.Unlock()
			if !inStore && !notified[k] {
				mu.Unlock()
				kv.Stop()
				t.Fatalf("trial %d: %q was deleted from the store without firing onExpire", trial, k)
			}
		}
		mu.Unlock()

		// A re-Put (not-expired) key must never be removed from the store nor
		// notified: it still holds a live session.
		for i := 0; i < reputs; i++ {
			k := fmt.Sprintf("r%02d", i)
			kv.mx.Lock()
			_, inStore := kv.kv[k]
			kv.mx.Unlock()
			mu.Lock()
			wasNotified := notified[k]
			mu.Unlock()
			if !inStore {
				kv.Stop()
				t.Fatalf("trial %d: re-Put key %q was wrongly removed from the store", trial, k)
			}
			if wasNotified {
				kv.Stop()
				t.Fatalf("trial %d: re-Put key %q was wrongly expired", trial, k)
			}
		}
		kv.Stop()
	}
}
