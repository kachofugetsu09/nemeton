package meeting

import (
	"fmt"
	"testing"
)

func TestCoordinatorQueueIsUnboundedAndDeduplicated(t *testing.T) {
	coordinator := NewCoordinator(nil)
	for index := 0; index < 1_000; index++ {
		coordinator.Enqueue(fmt.Sprintf("meeting-%d", index))
	}
	coordinator.Enqueue("meeting-0")
	if len(coordinator.pending) != 1_000 || len(coordinator.queued) != 1_000 {
		t.Fatalf("Coordinator queue pending=%d unique=%d", len(coordinator.pending), len(coordinator.queued))
	}
}
