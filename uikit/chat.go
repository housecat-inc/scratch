package uikit

type NewChatActionProps struct {
	Action  string
	Label   string
	Options []SelectOption
}

type SelectOption struct {
	Label string
	Value string
}
