package alerts

import (
	"testing"
	"time"
)

func TestDetailSelectionAlternatesAtLimitOneAndReservesRefreshCapacity(t *testing.T) {
	base := time.Unix(1000, 0)
	tasks := []detailTask{
		{id: "new-b", priority: 0, lastFetched: base.Add(-time.Minute)},
		{id: "new-a", priority: 0, lastFetched: base.Add(-2 * time.Minute)},
		{id: "refresh-b", priority: 1, lastFetched: base.Add(-3 * time.Hour)},
		{id: "refresh-a", priority: 1, lastFetched: base.Add(-4 * time.Hour)},
	}

	one := &Pipeline{maxPerCycle: 1}
	classes := []int{one.selectFair(tasks)[0].priority, one.selectFair(tasks)[0].priority, one.selectFair(tasks)[0].priority, one.selectFair(tasks)[0].priority}
	want := []int{0, 1, 0, 1}
	for i := range want {
		if classes[i] != want[i] {
			t.Fatalf("limit-one classes=%v", classes)
		}
	}

	four := (&Pipeline{maxPerCycle: 4}).selectFair(append(tasks,
		detailTask{id: "new-c", priority: 0}, detailTask{id: "new-d", priority: 0},
	))
	refreshes := 0
	for _, task := range four {
		if task.priority == 1 {
			refreshes++
		}
	}
	if len(four) != 4 || refreshes != 1 {
		t.Fatalf("weighted selection=%#v", four)
	}
	if four[0].id != "new-c" || four[1].id != "new-d" {
		t.Fatalf("new/changed ordering is not oldest then ID: %#v", four)
	}
}

func TestDetailSelectionBorrowsUnusedQuota(t *testing.T) {
	tasks := []detailTask{
		{id: "new", priority: 0},
		{id: "refresh-a", priority: 1},
		{id: "refresh-b", priority: 1},
		{id: "refresh-c", priority: 1},
	}
	selected := (&Pipeline{maxPerCycle: 4}).selectFair(tasks)
	if len(selected) != 4 {
		t.Fatalf("unused quota was not borrowed: %#v", selected)
	}
}
