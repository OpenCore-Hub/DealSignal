// Package crm defines the CRM integration interface and adapters.
// The NoOp adapter is the default; swap in HubSpot/Salesforce when configured.
package crm

import (
	"context"
	"time"
)

// Contact represents a person in the CRM.
type Contact struct {
	Email     string
	FirstName string
	Company   string
	Source    string // e.g. "deal_room_link", "file_request_link"
}

// Activity represents a tracked interaction.
type Activity struct {
	ContactEmail string
	Type         string // "link_opened", "file_downloaded", "question_asked", "file_uploaded"
	Description  string
	LinkName     string
	OccurredAt   time.Time
}

// DealStage represents CRM pipeline stages.
type DealStage string

const (
	DealStageAwareness   DealStage = "awareness"
	DealStageInterest    DealStage = "interest"
	DealStageEvaluation  DealStage = "evaluation"
	DealStageNegotiation DealStage = "negotiation"
	DealStageClosed      DealStage = "closed"
)

// Client is the CRM integration interface.
// All methods are idempotent and tolerate network failures gracefully.
type Client interface {
	// CreateOrUpdateContact ensures a contact exists in the CRM.
	// If the email already exists, metadata is merged.
	CreateOrUpdateContact(ctx context.Context, contact Contact) error

	// SyncActivity pushes an interaction to the CRM timeline.
	SyncActivity(ctx context.Context, activity Activity) error

	// UpdateDealStage moves a deal to a new pipeline stage.
	UpdateDealStage(ctx context.Context, email string, stage DealStage, notes string) error

	// HealthCheck returns nil if the CRM integration is reachable.
	HealthCheck(ctx context.Context) error
}
