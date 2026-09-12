package chat

import (
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageChatContext(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "ordinary chat"
		if linked {
			name = "page chat"
		}
		t.Run(name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			svc := newTestService(t, EchoAgent{})
			thread, err := svc.CreateThread("", "Edit page")
			r.NoError(err)
			if linked {
				r.NoError(svc.store.(db.PageStore).SetPageThread("example", thread.ID))
			}
			_, err = svc.Send(thread.ID, "Add a date filter")
			r.NoError(err)
			view := waitComplete(t, svc, thread.ID)
			r.Len(view.Messages, 2)
			a.Equal("Add a date filter", view.Messages[0].Body)
			if linked {
				a.Contains(view.Messages[1].Body, "/pages/example")
				a.Contains(view.Messages[1].Body, "Example Page")
			} else {
				a.NotContains(view.Messages[1].Body, "Scratch page")
			}
		})
	}
}
