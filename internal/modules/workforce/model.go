package workforce

import "time"

type StaffMember struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	InvitedAt   time.Time  `json:"invited_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

type StaffInput struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}
