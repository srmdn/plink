package server

import (
	"testing"

	"github.com/srmdn/plink/internal/db"
)

func TestOfferStatusSeparatesMissingAndPausedCardLink(t *testing.T) {
	today := "2026-11-10"
	offer := db.Offer{ItemType: "referral", Active: true, ProgramStatus: "active"}

	if got := offerStatus(offer, today); got != "needs homepage link" {
		t.Fatalf("offer without a card slug = %q, want needs homepage link", got)
	}

	offer.HomeSlug = "promo"
	if got := offerStatus(offer, today); got != "card link paused" {
		t.Fatalf("offer with an inactive card link = %q, want card link paused", got)
	}

	offer.HomeActive = true
	if got := offerStatus(offer, today); got != "active" {
		t.Fatalf("offer with an active card link = %q, want active", got)
	}
}
