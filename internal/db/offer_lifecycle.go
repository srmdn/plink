package db

import (
	"fmt"
	"time"
)

// LifecycleStatus controls all active slugs; Active controls catalog visibility.
// EndsOn is inclusive in the configured reporting timezone.
func (offer Offer) LifecycleStatus(today string) string {
	if offer.ProgramStatus == "ended" || offer.ProgramStatus == "paused" {
		return offer.ProgramStatus
	}
	if offer.EndsOn != "" && offer.EndsOn < today {
		return "expired"
	}
	if offer.StartsOn != "" && offer.StartsOn > today {
		return "upcoming"
	}
	return "active"
}

func normalizeOfferLifecycle(input OfferInput) OfferInput {
	if input.ItemType == "" {
		input.ItemType = "referral"
	}
	if input.ProgramStatus == "" {
		input.ProgramStatus = "active"
	}
	if input.EndedBehavior == "" {
		input.EndedBehavior = "notice"
	}
	return input
}

// GetOfferLifecycle reads only the metadata needed by a public slug request.
func (db *DB) GetOfferLifecycle(id int64) (*Offer, error) {
	var offer Offer
	err := db.QueryRow(`SELECT id, title, starts_on, ends_on, fallback_url,
		program_status, ended_behavior, notice_message, notice_source_url, status_changed_on, verified_on
		FROM offers WHERE id = ?`, id).Scan(&offer.ID, &offer.Title, &offer.StartsOn, &offer.EndsOn, &offer.FallbackURL,
		&offer.ProgramStatus, &offer.EndedBehavior, &offer.NoticeMessage, &offer.NoticeSourceURL, &offer.StatusChangedOn, &offer.VerifiedOn)
	if err != nil {
		return nil, err
	}
	return &offer, nil
}

// Notice views never enter redirect-click analytics or count as conversions.
func (db *DB) RecordNoticeView(linkID int64, status, referrer, userAgent string, viewedAt int64) error {
	_, err := db.Exec(`INSERT INTO notice_views (link_id, viewed_at, program_status, referrer, user_agent)
		VALUES (?, ?, ?, ?, ?)`, linkID, viewedAt, status, referrer, userAgent)
	return err
}

// Attach an existing standalone channel slug without moving another Offer's link.
func (db *DB) AttachExistingOfferLink(offerID int64, slug string) error {
	result, err := db.Exec(`UPDATE links SET offer_id = ?, updated_at = ?
		WHERE slug = ? AND offer_id IS NULL AND active = 1
		AND EXISTS (SELECT 1 FROM offers WHERE id = ?)`, offerID, time.Now().Unix(), slug, offerID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("existing slug must be active and not already attached to an offer")
	}
	return nil
}
