package main

import (
	"net/url"
	"path"
	"strings"

	"github.com/AlbinoDrought/creamy-inbound-stuff/stuff"
)

type ChallengeURLGenerator interface {
	ViewChallenge(challenge *stuff.Challenge) string
	UploadToChallenge(challenge *stuff.Challenge) string
}

type BrowseURLGenerator interface {
	BrowsePath(filePath string) string
	SharePath(filePath string) string
}

func aftermarketEscape(url string) string {
	return strings.ReplaceAll(url, "=", "%3D")
}

type hardcodedURLGenerator struct{}

func (generator *hardcodedURLGenerator) ViewChallenge(challenge *stuff.Challenge) string {
	challengeURL := url.URL{Path: "/upload/" + challenge.ID}
	return aftermarketEscape(challengeURL.String())
}

func (generator *hardcodedURLGenerator) UploadToChallenge(challenge *stuff.Challenge) string {
	challengeURL := url.URL{Path: "/upload/" + challenge.ID + "/file"}
	return aftermarketEscape(challengeURL.String())
}

func (generator *hardcodedURLGenerator) BrowsePath(filePath string) string {
	browseURL := url.URL{Path: "/stuff/browse" + path.Clean(filePath)}
	return browseURL.String()
}

func (generator *hardcodedURLGenerator) SharePath(filePath string) string {
	browseURL := url.URL{Path: "/stuff/share" + path.Clean(filePath)}
	return browseURL.String()
}
