package worker

import "testing"

func TestNewPool(t *testing.T) {
	pool := NewPool(4, 5)

	if pool.PoolSize != 4 {
		t.Errorf("PoolSize = %d, want 4", pool.PoolSize)
	}
	if pool.PollInterval != 5 {
		t.Errorf("PollInterval = %d, want 5", pool.PollInterval)
	}
	if len(pool.Workers) != 4 {
		t.Errorf("len(Workers) = %d, want 4", len(pool.Workers))
	}
	if pool.LogChannel == nil {
		t.Error("LogChannel should not be nil")
	}

	// Workers must be numbered 1 through N.
	for i, w := range pool.Workers {
		expectedNumber := i + 1
		if w.Number != expectedNumber {
			t.Errorf("Workers[%d].Number = %d, want %d", i, w.Number, expectedNumber)
		}
	}
}

func TestNewPoolSingleWorker(t *testing.T) {
	pool := NewPool(1, 10)

	if len(pool.Workers) != 1 {
		t.Fatalf("len(Workers) = %d, want 1", len(pool.Workers))
	}
	if pool.Workers[0].Number != 1 {
		t.Errorf("Workers[0].Number = %d, want 1", pool.Workers[0].Number)
	}
}

func TestGetWorker(t *testing.T) {
	pool := NewPool(3, 5)

	w1 := pool.GetWorker(1)
	if w1 == nil {
		t.Fatal("GetWorker(1) returned nil, want worker 1")
	}
	if w1.Number != 1 {
		t.Errorf("GetWorker(1).Number = %d, want 1", w1.Number)
	}

	w3 := pool.GetWorker(3)
	if w3 == nil {
		t.Fatal("GetWorker(3) returned nil, want worker 3")
	}
	if w3.Number != 3 {
		t.Errorf("GetWorker(3).Number = %d, want 3", w3.Number)
	}

	// Out-of-range lookups must return nil.
	if pool.GetWorker(0) != nil {
		t.Error("GetWorker(0) should return nil")
	}
	if pool.GetWorker(4) != nil {
		t.Error("GetWorker(4) should return nil for a pool of size 3")
	}
	if pool.GetWorker(-1) != nil {
		t.Error("GetWorker(-1) should return nil")
	}
}
