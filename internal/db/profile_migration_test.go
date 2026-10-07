package db

import (
	"path/filepath"
	"testing"
)

func TestArticleItemTypeAndProfileSettings(t *testing.T) {
	database, err := Init(filepath.Join(t.TempDir(), "article.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	offer, err := database.CreateOfferWithHomepageLink(OfferInput{Title: "Tulisan", ItemType: "article", Active: true}, "tulisan", "https://blog.example/tulisan")
	if err != nil {
		t.Fatalf("create article offer: %v", err)
	}
	if offer.ItemType != "article" || offer.HomeURL != "https://blog.example/tulisan" {
		t.Fatalf("article offer not stored: %+v", offer)
	}

	settings, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.AvatarURL != "" || settings.SocialLinks != "" {
		t.Fatalf("profile defaults not empty: avatar=%q social=%q", settings.AvatarURL, settings.SocialLinks)
	}

	settings.AvatarURL = "/media/avatar.png"
	settings.SocialLinks = `[{"platform":"instagram","url":"https://instagram.com/said"}]`
	if err := database.SaveSiteSettings(settings); err != nil {
		t.Fatal(err)
	}
	saved, err := database.GetSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.AvatarURL != settings.AvatarURL || saved.SocialLinks != settings.SocialLinks {
		t.Fatalf("profile settings not persisted: %+v", saved)
	}
}
