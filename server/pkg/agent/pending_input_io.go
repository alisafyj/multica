package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

const pendingInputWaitHeartbeatInterval = 5 * time.Second

var errPendingInputDeliveryFailed = errors.New("failed to acknowledge delivered user input")

const (
	pendingInputMaxActiveRequests    = 3
	pendingInputMaxCompletedRequests = 32
)

type pendingInputBeginResult int

const (
	pendingInputAccepted pendingInputBeginResult = iota + 1
	pendingInputDuplicate
	pendingInputConflict
	pendingInputLimitReached
)

type pendingInputRegistryEntry struct {
	requestKey  string
	payloadHash [sha256.Size]byte
	completed   bool
}

type pendingInputCoordinator struct {
	mu            sync.Mutex
	entries       map[string]pendingInputRegistryEntry
	requestKeys   map[string]string
	active        int
	completed     int
	deliveryError error
	wg            sync.WaitGroup
}

func newPendingInputCoordinator() *pendingInputCoordinator {
	return &pendingInputCoordinator{
		entries:     make(map[string]pendingInputRegistryEntry),
		requestKeys: make(map[string]string),
	}
}

func (c *pendingInputCoordinator) begin(nativeID, requestKey string, payloadHash [sha256.Size]byte) pendingInputBeginResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, exists := c.entries[nativeID]; exists {
		if entry.requestKey == requestKey && entry.payloadHash == payloadHash {
			return pendingInputDuplicate
		}
		return pendingInputConflict
	}
	if _, exists := c.requestKeys[requestKey]; exists {
		return pendingInputDuplicate
	}
	if c.active >= pendingInputMaxActiveRequests || c.completed >= pendingInputMaxCompletedRequests {
		return pendingInputLimitReached
	}
	c.entries[nativeID] = pendingInputRegistryEntry{requestKey: requestKey, payloadHash: payloadHash}
	c.requestKeys[requestKey] = nativeID
	c.active++
	c.wg.Add(1)
	return pendingInputAccepted
}

func pendingInputPayloadHash(request PendingInputRequest) [sha256.Size]byte {
	encoded, _ := json.Marshal(request)
	return sha256.Sum256(encoded)
}

func (c *pendingInputCoordinator) finish(nativeID string) {
	c.mu.Lock()
	entry, exists := c.entries[nativeID]
	if exists && !entry.completed {
		entry.completed = true
		c.entries[nativeID] = entry
		c.active--
		c.completed++
		c.wg.Done()
	}
	c.mu.Unlock()
}

func (c *pendingInputCoordinator) recordDeliveryFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deliveryError = errPendingInputDeliveryFailed
}

func (c *pendingInputCoordinator) wait() error {
	c.wg.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deliveryError
}

func runPendingInputCallback(ctx context.Context, callback func(context.Context, PendingInputRequest) (PendingInputAnswer, error), request PendingInputRequest, heartbeat func()) (PendingInputAnswer, error) {
	done := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(pendingInputWaitHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				heartbeat()
			}
		}
	}()
	answer, err := callback(ctx, request)
	close(done)
	<-heartbeatDone
	return answer, err
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(data)
}

func writeJSONLine(w interface{ Write([]byte) (int, error) }, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}
