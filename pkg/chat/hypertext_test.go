package chat

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

var screenshots = []testkit.Screenshot{
	{Height: 480, Name: "chat.png", Width: 640},
}

func TestChat(t *testing.T) {
	run(t, []Case{
		{
			Name: "empty index offers a new chat",
			Path: "/chat",
			Assert: []Step{
				TextContains(".chatapp", "No chats yet"),
				Visible("#new-thread"),
			},
		},
		{
			Name: "starts a chat and streams a reply over sse",
			Path: "/chat",
			Act: []Step{
				Click("#new-thread"),
				Type("#chat-input", "hi agent"),
				Press("#chat-input", "Enter"),
			},
			Assert: []Step{
				TextContains("#chat-messages", "hi agent"),
				TextContains("#chat-messages", "You said: hi agent"),
				ClassContains("#chat-messages .role-assistant", "status-complete"),
				Log(`{"level":"INFO","method":"POST","msg":"http.request","path":"/chat/1/messages","status":204}`),
			},
		},
		{
			Name:        "lists threads and opens a conversation",
			Path:        "/chat",
			Screenshots: screenshots,
			Seed: []Step{
				SeedThread(""),
				SeedExchange(1, "what is on my list today?"),
			},
			Act: []Step{
				Click(".thread-list a"),
			},
			Assert: []Step{
				TextContains("#thread-heading", "what is on my list today?"),
				TextContains("#chat-messages", "You said: what is on my list today?"),
			},
		},
		{
			Name:        "renders a conversation",
			Path:        "/chat/1",
			Screenshots: []testkit.Screenshot{{Height: 480, Name: "thread.png", Width: 640}},
			Seed: []Step{
				SeedThread(""),
				SeedExchange(1, "hello"),
			},
			Assert: []Step{
				TextContains("#chat-messages", "You said: hello"),
			},
		},
		{
			Name:  "agent failure renders an error bubble",
			Agent: failingAgent{},
			Path:  "/chat",
			Seed:  []Step{SeedThread("doomed")},
			Act: []Step{
				Click(".thread-list a"),
				Type("#chat-input", "boom"),
				Press("#chat-input", "Enter"),
			},
			Assert: []Step{
				ClassContains("#chat-messages .role-assistant", "status-error"),
				TextContains("#chat-messages", "agent exploded"),
				Log(`{"error":"agent exploded","level":"ERROR","message":2,"msg":"chat.run","thread":1}`),
			},
		},
	})
}
