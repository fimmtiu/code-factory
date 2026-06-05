package ui

import (
	"testing"

	"github.com/fimmtiu/code-factory/internal/models"
)

func TestStillActionable_NilDBOrEmptyIdentifier(t *testing.T) {
	if !(Model{}).stillActionable("proj/t1") {
		t.Error("nil db should err toward showing the notification")
	}
	m := NewModel(nil, openUITestDB(t), 5)
	if !m.stillActionable("") {
		t.Error("empty identifier should err toward showing the notification")
	}
}

func TestStillActionable_UnknownTicket(t *testing.T) {
	m := NewModel(nil, openUITestDB(t), 5)
	if !m.stillActionable("proj/missing") {
		t.Error("unknown ticket should err toward showing the notification")
	}
}

func TestStillActionable_ByStatus(t *testing.T) {
	cases := []struct {
		status models.TicketStatus
		want   bool
	}{
		{models.StatusNeedsAttention, true},
		{models.StatusUserReview, true},
		{models.StatusWorking, false},
		{models.StatusResponding, false},
		{models.StatusIdle, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			d := openUITestDB(t)
			if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
				t.Fatal(err)
			}
			if err := d.CreateTicket("proj/t1", "T", nil, "", nil); err != nil {
				t.Fatal(err)
			}
			if err := d.SetStatus("proj/t1", models.PhaseImplement, tc.status); err != nil {
				t.Fatal(err)
			}
			m := NewModel(nil, d, 5)
			if got := m.stillActionable("proj/t1"); got != tc.want {
				t.Errorf("stillActionable(%s) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
