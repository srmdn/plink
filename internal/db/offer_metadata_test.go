package db

import (
	"path/filepath"
	"testing"
)

func TestUpdateOfferPreservesAttachedLinkMetadata(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "plink.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	home, err := database.CreateLinkWithOptions("existing", "https://example.com/home", "Original description", "Original category", LinkOptions{Provider: "Original provider", Channel: "blog", Campaign: "launch"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := database.CreateLinkWithOptions("other", "https://example.com/other", "Other description", "Other category", LinkOptions{Provider: "Other provider", Channel: "social", Campaign: "campaign"})
	if err != nil {
		t.Fatal(err)
	}
	input := OfferInput{Title: "Offer", Description: "Card description", Category: "Card category", Provider: "Card provider", Active: true}
	offer, err := database.CreateOfferWithExistingHomepageLink(input, home.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AttachExistingOfferLink(offer.ID, other.Slug); err != nil {
		t.Fatal(err)
	}
	if err := database.RecordClick(home.ID, "", ""); err != nil {
		t.Fatal(err)
	}

	input.Active = false
	input.Description = "Changed card description"
	input.Category = "Changed card category"
	input.Provider = "Changed card provider"
	if err := database.UpdateOffer(offer.ID, input); err != nil {
		t.Fatal(err)
	}
	updatedOffer, err := database.GetOfferByID(offer.ID)
	if err != nil || updatedOffer == nil {
		t.Fatalf("get updated offer: %v", err)
	}
	if updatedOffer.Active || updatedOffer.Description != input.Description || updatedOffer.Category != input.Category || updatedOffer.Provider != input.Provider {
		t.Fatalf("offer update not saved: %+v", updatedOffer)
	}
	for _, want := range []*Link{home, other} {
		got, err := database.GetLinkByID(want.ID)
		if err != nil || got == nil {
			t.Fatalf("get link %d: %v", want.ID, err)
		}
		if got.Slug != want.Slug || got.URL != want.URL || got.Description != want.Description || got.Category != want.Category || got.Provider != want.Provider || got.Channel != want.Channel || got.Campaign != want.Campaign || !got.Active || got.OfferID != offer.ID {
			t.Errorf("link metadata changed: got %+v; original %+v", got, want)
		}
		if want.ID == home.ID && got.Clicks != 1 {
			t.Errorf("click history = %d, want 1", got.Clicks)
		}
	}
}
