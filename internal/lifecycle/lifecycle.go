// Package lifecycle names the statuses of the records several modules read. Each block mirrors a
// CHECK constraint in the migrations, so a status is spelled once in Go; what may follow what
// (the workflow) stays in the module that owns the record.
package lifecycle

// Orders (orders.status). A status is also a monotonic concurrency token: final orders cannot be
// reopened. The workflow lives in modules/ordering.
const (
	OrderDraft     = "draft"
	OrderNew       = "new"
	OrderPreparing = "preparing"
	OrderServed    = "served"
	OrderCompleted = "completed"
	OrderCancelled = "cancelled"
)

// Order channels (orders.channel).
const (
	ChannelDineIn   = "dine_in"
	ChannelOnline   = "online"
	ChannelPreorder = "preorder"
)

// Reservations (reservations.status). A guest booking waits as pending while its hold is open.
const (
	ReservationPending   = "pending"
	ReservationConfirmed = "confirmed"
	ReservationSeated    = "seated"
	ReservationCompleted = "completed"
	ReservationCancelled = "cancelled"
	ReservationNoShow    = "no_show"
)

// Bills (bills.status). Guests may add orders only to an open bill.
const (
	BillOpen           = "open"
	BillPaymentPending = "payment_pending"
	BillPaid           = "paid"
	BillVoid           = "void"
)

// Payments (payments.status).
const (
	PaymentPending        = "pending"
	PaymentRequiresAction = "requires_action"
	PaymentProcessing     = "processing"
	PaymentPaid           = "paid"
	PaymentFailed         = "failed"
	PaymentRefunded       = "refunded"
	PaymentDisputed       = "disputed"
	PaymentCancelled      = "cancelled"
)

// Venue team memberships (venue_staff.status). Removed members are kept for history and never
// set through the API.
const (
	StaffInvited  = "invited"
	StaffActive   = "active"
	StaffInactive = "inactive"
	StaffRemoved  = "removed"
)

// StaffStatuses are the membership statuses an owner may set.
var StaffStatuses = []string{StaffInvited, StaffActive, StaffInactive}

// Venues (venues.status). Only active venues are public in Dish.
const (
	VenueDraft  = "draft"
	VenueActive = "active"
	VenuePaused = "paused"
	VenueClosed = "closed"
)

var VenueStatuses = []string{VenueDraft, VenueActive, VenuePaused, VenueClosed}

// Workspace subscriptions (workspace_subscriptions).
const (
	SubscriptionTrialing  = "trialing"
	SubscriptionActive    = "active"
	SubscriptionPastDue   = "past_due"
	SubscriptionCancelled = "cancelled"
	SubscriptionExpired   = "expired"

	PlanStart    = "start"
	PlanBusiness = "business"
	PlanPro      = "pro"

	CycleMonthly = "monthly"
	CycleYearly  = "yearly"
)

var (
	Plans         = []string{PlanStart, PlanBusiness, PlanPro}
	BillingCycles = []string{CycleMonthly, CycleYearly}
)
