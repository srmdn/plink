package db

import (
	"path/filepath"
	"testing"
)

// TestGetOfferByHomeSlugReadsOneOffer verifies the direct slug lookup returns
// the same full projection as ListOffers: product details, counts, and the
// homepage link, without requiring the caller to scan every offer.
func TestGetOfferByHomeSlugReadsOneOffer(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "slug.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	price := int64(125000)
	created, err := database.CreateOfferWithHomepageLink(OfferInput{
		Title:         "Product card",
		ItemType:      "product",
		Active:        true,
		ProgramStatus: "active",
		Product: &ProductDetails{
			Enabled: true, Kind: "digital", PriceMinor: &price, Currency: "IDR",
			Details: "What you get", Fulfillment: "Instant download",
			WhatsApp: &ProductWhatsApp{Enabled: true, Number: "+6281234567890", Message: "Hi"},
		},
	}, "product-card", "https://provider.example/?ref=card")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreateOfferLink(created.ID, "product-card-social", "https://provider.example/?ref=social", "social", "launch"); err != nil {
		t.Fatal(err)
	}

	offer, err := database.GetOfferByHomeSlug("product-card")
	if err != nil {
		t.Fatal(err)
	}
	if offer == nil {
		t.Fatal("slug lookup returned no offer")
	}
	if offer.ID != created.ID || offer.ItemType != "product" || offer.ProductPage != true {
		t.Fatalf("offer identity = %#v", offer)
	}
	if offer.ProductDetails != "What you get" || offer.Fulfillment != "Instant download" ||
		offer.PriceMinor == nil || *offer.PriceMinor != price || offer.Currency != "IDR" {
		t.Fatalf("product details not projected: %#v", offer)
	}
	if !offer.WhatsAppEnabled || offer.WhatsAppNumber != "+6281234567890" || offer.LinkCount != 2 {
		t.Fatalf("counts or WhatsApp not projected: %#v", offer)
	}

	// A paused card link must still resolve: visibility belongs to the caller.
	links, err := database.ListOfferLinks(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	var homeLinkID int64
	for _, link := range links {
		if link.OfferHome {
			homeLinkID = link.ID
		}
	}
	if homeLinkID == 0 {
		t.Fatalf("homepage link missing from %#v", links)
	}
	if err := database.ToggleLink(homeLinkID); err != nil {
		t.Fatal(err)
	}
	pausedLink, err := database.GetOfferByHomeSlug("product-card")
	if err != nil || pausedLink == nil || pausedLink.HomeActive {
		t.Fatalf("paused card link lookup = %#v, %v", pausedLink, err)
	}

	// A paused card must still resolve too: visibility belongs to the caller.
	if err := database.ToggleOffer(created.ID); err != nil {
		t.Fatal(err)
	}
	paused, err := database.GetOfferByHomeSlug("product-card")
	if err != nil || paused == nil || paused.Active {
		t.Fatalf("paused lookup = %#v, %v", paused, err)
	}

	if missing, err := database.GetOfferByHomeSlug("no-such-card"); err != nil || missing != nil {
		t.Fatalf("unknown slug = %#v, %v", missing, err)
	}
	if empty, err := database.GetOfferByHomeSlug(""); err != nil || empty != nil {
		t.Fatalf("empty slug = %#v, %v", empty, err)
	}
}
