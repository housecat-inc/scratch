package wire

// FailureTags are failure tag names as a hand-labels review form renders
// them for its script (ui/dom/hand_labels_review.ts): the tags the workspace
// knows, and the ones a review has.
type FailureTags []string

// FailureReviewOut is /evals/:id/failure-review's answer: the revision the
// review was saved at, which the form's next save builds on.
type FailureReviewOut struct {
	Revision int64 `json:"revision"`
}

// HandLabelSaveOut is /internal/labeling/:id/labels's answer to an autosave:
// the label saved, whose id the form's next save builds on, and its state;
// with apply_sender, how many other emails from the sender took it, or why
// they did not all.
type HandLabelSaveOut struct {
	AppliedToSender  int    `json:"applied_to_sender,omitempty"`
	ID               int64  `json:"id"`
	SenderApplyError string `json:"sender_apply_error,omitempty"`
	State            string `json:"state"`
}

// HandLabelTags are the tags of an email being labelled, as the label form
// renders them for its script (ui/dom/hand_labels.ts) and posts them back in
// its tags field.
type HandLabelTags []string
