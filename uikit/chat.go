package uikit

type NewChatActionProps struct {
	Action  string
	Options []SelectOption
}

type SelectOption struct {
	Label string
	Value string
}
