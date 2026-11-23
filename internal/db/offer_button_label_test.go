package db

import (
	"path/filepath"
	"testing"
)

// TestOfferButtonLabelDefaultsByItemType covers the card action text when the
// operator leaves the button label empty: each item type gets an action that
// matches what the card does, and an explicit label is never replaced.
func TestOfferButtonLabelDefaultsByItemType(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "plink.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	cases := []struct {
		itemType string
		want     string
	}{
		{"referral", "View offer"},
		{"product", "View product"},
		{"service", "View service"},
		{"resource", "Open link"},
		{"article", "Read article"},
		{"project", "View project"},
	}
	for _, tc := range cases {
		offer, err := database.CreateOfferWithHomepageLink(OfferInput{
			Title: "Item " + tc.itemType, ItemType: tc.itemType, Active: true,
		}, "item-"+tc.itemType, "https://example.com/"+tc.itemType)
		if err != nil {
			t.Fatalf("%s: %v", tc.itemType, err)
		}
		if offer.ButtonLabel != tc.want {
			t.Errorf("%s label = %q, want %q", tc.itemType, offer.ButtonLabel, tc.want)
		}
	}

	offer, err := database.CreateOfferWithHomepageLink(OfferInput{
		Title: "Explicit", ItemType: "project", Active: true, ButtonLabel: "Open the repo",
	}, "item-explicit", "https://example.com/repo")
	if err != nil {
		t.Fatal(err)
	}
	if offer.ButtonLabel != "Open the repo" {
		t.Fatalf("explicit label was replaced: %q", offer.ButtonLabel)
	}

	// Clearing the label on an edit falls back to the type default again.
	if err := database.UpdateOffer(offer.ID, OfferInput{
		Title: offer.Title, ItemType: "project", Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := database.GetOfferByID(offer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ButtonLabel != "View project" {
		t.Fatalf("cleared label = %q, want %q", updated.ButtonLabel, "View project")
	}
}
