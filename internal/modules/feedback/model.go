package feedback

import "time"

type Review struct {
	ID         string     `json:"id"`
	GuestName  string     `json:"guest_name"`
	Rating     int        `json:"rating"`
	Table      string     `json:"table,omitempty"`
	Order      string     `json:"order,omitempty"`
	Body       string     `json:"body,omitempty"`
	OwnerReply string     `json:"owner_reply,omitempty"`
	RepliedAt  *time.Time `json:"replied_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
