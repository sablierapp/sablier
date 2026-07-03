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

		for i := 0; i < survivors+2*reputs+5; i++ {
			kv.expireFunc()
		}
		time.Sleep(15 * time.Millisecond) // let async notifyExpirations run

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

		// A re-Put (not-expired) key must never be notified or removed.
		for i := 0; i < reputs; i++ {
			k := fmt.Sprintf("r%02d", i)
			mu.Lock()
			wrong := notified[k]
			mu.Unlock()
			if wrong {
				kv.Stop()
				t.Fatalf("trial %d: re-Put key %q was wrongly expired", trial, k)
			}
		}
		kv.Stop()
	}
}
