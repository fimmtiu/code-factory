package ui

import (
	"errors"

	"github.com/fimmtiu/tickets/internal/models"
)

var errTestError = errors.New("test error")

// sampleUnits returns a small set of WorkUnit pointers for use in tests.
func sampleUnits() []*models.WorkUnit {
	proj := models.NewProject("myproject", "My Project")
	proj.Status = models.ProjectOpen

	sub := models.NewProject("myproject/subproject", "Sub Project")
	sub.Status = models.ProjectInProgress
	sub.Parent = "myproject"

	t1 := models.NewTicket("myproject/ticket-one", "First ticket")
	t1.Status = models.StatusOpen
	t1.Parent = "myproject"

	t2 := models.NewTicket("myproject/subproject/ticket-two", "Second ticket")
	t2.Status = models.StatusInProgress
	t2.Parent = "myproject/subproject"

	t3 := models.NewTicket("myproject/ticket-three", "Third ticket")
	t3.Status = models.StatusDone
	t3.Parent = "myproject"

	return []*models.WorkUnit{proj, sub, t1, t2, t3}
}
