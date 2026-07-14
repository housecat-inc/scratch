package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestInboxTasksBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				Click("[data-new-chat]"),
			},
			Assert: []Step{
				ChatThreadCount(1),
				ElementEventuallyPresent("#floating-chat [data-chat-input]"),
				TextContains("#floating-chat", "New chat"),
			},
			Name: "new chat action opens a floating chat",
			Path: "/",
		},
		{
			Act: []Step{
				SelectOption(`[name="provider_model"]`, "echo:default"),
			},
			Assert: []Step{
				ChatThreadCount(1),
				ElementEventuallyPresent("#floating-chat [data-chat-input]"),
				TextContains("#floating-chat", "New chat"),
			},
			Name: "provider selection opens a floating chat",
			Path: "/",
		},
		{
			Act: []Step{
				Type("#chat-input", "hello from draft"),
				Click("#chat-send"),
			},
			Assert: []Step{
				ChatThreadCount(1),
				TextContains(".mail-reader-title", "hello from draft"),
			},
			Name: "sending from a draft chat creates the thread",
			Path: "/inbox/chats/new",
		},
		{
			Act: []Step{
				Type("#chat-input", "buy milk"),
				Click("#chat-send"),
			},
			Assert: []Step{
				ChatThreadCount(1),
				TextContains(".mail-reader-title", "buy milk"),
			},
			Name: "footer composer always starts a chat",
			Path: "/inbox/tasks",
		},
		{
			Assert: []Step{
				TextContains(".mail-list", "active one"),
				ElementAbsent(`[data-id="2"]`),
				ElementAbsent(`[data-id="3"]`),
			},
			Name: "active filter excludes done and archived tasks",
			Path: "/inbox/tasks?archived=active",
			Seed: []Step{
				SeedTask("active one"),
				SeedTask("done one", TaskCompleted(true)),
				SeedTask("archived one", TaskArchived(true)),
			},
		},
		{
			Act: []Step{
				Type(`[name="description"]`, "Call customer before Friday"),
				Click(".mail-save-btn"),
			},
			Assert: []Step{
				TaskDescription(1, "Call customer before Friday"),
				TextContains(".mail-reader-title", "draft"),
			},
			Name: "updates the task description",
			Path: "/inbox/tasks/1",
			Seed: []Step{
				SeedTask("draft"),
			},
		},
	})
}

func ChatThreadCount(want int) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			threads, err := h.Chat.Threads()
			return err == nil && len(threads) == want
		}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
	}
}

func TaskCount(want int) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			tasks, err := h.Tasks.All()
			return err == nil && len(tasks) == want
		}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
	}
}
