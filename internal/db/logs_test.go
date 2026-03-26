package db_test

import (
	"testing"
)

func TestInsertLog_AndGetLogs(t *testing.T) {
	d, _, _ := openTestDB(t)

	if err := d.InsertLog(1, "hello", ""); err != nil {
		t.Fatalf("InsertLog: %v", err)
	}
	if err := d.InsertLog(2, "world", "/tmp/log.txt"); err != nil {
		t.Fatalf("InsertLog: %v", err)
	}

	entries, err := d.GetLogs()
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	if entries[0].WorkerNumber != 1 {
		t.Errorf("entry[0].WorkerNumber: got %d, want 1", entries[0].WorkerNumber)
	}
	if entries[0].Message != "hello" {
		t.Errorf("entry[0].Message: got %q, want %q", entries[0].Message, "hello")
	}
	if entries[0].Logfile != "" {
		t.Errorf("entry[0].Logfile: got %q, want empty", entries[0].Logfile)
	}

	if entries[1].WorkerNumber != 2 {
		t.Errorf("entry[1].WorkerNumber: got %d, want 2", entries[1].WorkerNumber)
	}
	if entries[1].Message != "world" {
		t.Errorf("entry[1].Message: got %q, want %q", entries[1].Message, "world")
	}
	if entries[1].Logfile != "/tmp/log.txt" {
		t.Errorf("entry[1].Logfile: got %q, want %q", entries[1].Logfile, "/tmp/log.txt")
	}
}

func TestGetLogs_EmptyReturnsNil(t *testing.T) {
	d, _, _ := openTestDB(t)

	entries, err := d.GetLogs()
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries on empty DB, got %d", len(entries))
	}
}

func TestInsertLog_PrunesOldestBeyond200(t *testing.T) {
	d, _, _ := openTestDB(t)

	// Insert 205 log entries
	for i := 0; i < 205; i++ {
		msg := "msg"
		if i == 0 {
			msg = "oldest"
		}
		if err := d.InsertLog(i%10, msg, ""); err != nil {
			t.Fatalf("InsertLog %d: %v", i, err)
		}
	}

	entries, err := d.GetLogs()
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}

	if len(entries) != 200 {
		t.Errorf("expected exactly 200 log entries after pruning, got %d", len(entries))
	}

	// The oldest 5 entries should have been deleted; "oldest" was entry 0.
	for _, e := range entries {
		if e.Message == "oldest" {
			t.Error("expected oldest entry to have been pruned, but it is still present")
		}
	}
}

func TestInsertLog_ExactlyAtLimit(t *testing.T) {
	d, _, _ := openTestDB(t)

	// Insert exactly 200 entries
	for i := 0; i < 200; i++ {
		if err := d.InsertLog(1, "msg", ""); err != nil {
			t.Fatalf("InsertLog %d: %v", i, err)
		}
	}

	entries, err := d.GetLogs()
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(entries) != 200 {
		t.Errorf("expected 200 entries, got %d", len(entries))
	}
}

func TestGetLogs_OrderedByTimestampAsc(t *testing.T) {
	d, _, _ := openTestDB(t)

	msgs := []string{"first", "second", "third"}
	for i, msg := range msgs {
		if err := d.InsertLog(i+1, msg, ""); err != nil {
			t.Fatalf("InsertLog: %v", err)
		}
	}

	entries, err := d.GetLogs()
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if len(entries) < 2 {
		return
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.Before(entries[i-1].Timestamp) {
			t.Errorf("entries not in ascending order at index %d", i)
		}
	}
}
