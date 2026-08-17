// Package emailid canonicalizes account emails and rejects disposable inboxes
// so one mailbox cannot farm a 14-day trial per plus-tag or throwaway domain.
package emailid

import (
	"strings"
)

// Canonical returns a uniqueness key for an account email.
// Gmail dots and plus-tags collapse to one mailbox; other providers only
// strip plus-tags (RFC 5233 subaddressing usually delivers to the same inbox).
func Canonical(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" {
		return email
	}
	if i := strings.IndexByte(local, '+'); i >= 0 {
		local = local[:i]
	}
	if domain == "googlemail.com" {
		domain = "gmail.com"
	}
	if domain == "gmail.com" {
		local = strings.ReplaceAll(local, ".", "")
	}
	return local + "@" + domain
}

// SameMailbox reports whether two addresses collapse to the same account key.
func SameMailbox(a, b string) bool {
	left, right := Canonical(a), Canonical(b)
	return left != "" && left == right
}

// Keys returns lookup forms for an address: lowercased raw, then Canonical
// when that differs (Gmail dots/plus, plus-tags on other providers).
func Keys(email string) []string {
	raw := strings.ToLower(strings.TrimSpace(email))
	if raw == "" {
		return nil
	}
	out := []string{raw}
	if c := Canonical(email); c != "" && c != raw {
		out = append(out, c)
	}
	return out
}

// IsDisposable reports whether the domain is a known throwaway inbox provider.
func IsDisposable(email string) bool {
	_, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok || domain == "" {
		return false
	}
	if _, ok := disposableDomains[domain]; ok {
		return true
	}
	for _, suffix := range disposableSuffixes {
		if strings.HasSuffix(domain, suffix) {
			return true
		}
	}
	return false
}

var disposableSuffixes = []string{
	".anonaddy.com",
	".anonaddy.me",
	".mailinator.com",
	".simplelogin.com",
	".simplelogin.co",
}

var disposableDomains = map[string]struct{}{
	"0-mail.com": {}, "10minutemail.com": {}, "10minutemail.net": {},
	"14minutetest.com": {}, "1secmail.com": {}, "1secmail.org": {},
	"33mail.com": {}, "guerrillamail.com": {}, "guerrillamail.net": {},
	"guerrillamail.org": {}, "guerrillamailblock.com": {}, "sharklasers.com": {},
	"grr.la": {}, "guerrillamail.biz": {}, "guerrillamail.de": {},
	"mailinator.com": {}, "mailinator.net": {}, "mailinator.org": {},
	"mailinator2.com": {}, "maildrop.cc": {}, "mailnesia.com": {},
	"mailcatch.com": {}, "mailnull.com": {}, "mailinator.co.uk": {},
	"temp-mail.org": {}, "temp-mail.io": {}, "tempmail.com": {},
	"tempmail.net": {}, "tempmailo.com": {}, "tempail.com": {},
	"throwawaymail.com": {}, "trashmail.com": {}, "trashmail.net": {},
	"trashmailer.com": {}, "yopmail.com": {}, "yopmail.fr": {},
	"yopmail.net": {}, "getnada.com": {}, "nada.email": {},
	"discard.email": {}, "dispostable.com": {}, "fakeinbox.com": {},
	"fakemailgenerator.com": {}, "getairmail.com": {}, "inboxkitten.com": {},
	"moakt.com": {}, "mohmal.com": {}, "mytemp.email": {},
	"tempinbox.com": {}, "tmpmail.org": {}, "tmpmail.net": {},
	"emailondeck.com": {}, "generator.email": {}, "harakirimail.com": {},
	"jetable.org": {}, "meltmail.com": {}, "mintemail.com": {},
	"mytrashmail.com": {}, "nowmymail.com": {}, "putthisinyourspamdatabase.com": {},
	"spamgourmet.com": {}, "spamherelots.com": {}, "tempmailaddress.com": {},
	"thankyou2010.com": {}, "thisisnotmyrealemail.com": {}, "veryrealemail.com": {},
	"wegwerfmail.de": {}, "wegwerfmail.net": {}, "einrot.com": {},
	"mailforspam.com": {}, "spam4.me": {}, "mailinator.us": {},
	"guerrillamail.info": {}, "pokemail.net": {}, "spamhereplease.com": {},
	"tempmailer.com": {}, "throwawayemailaddress.com": {}, "tmpeml.com": {},
	"dropmail.me": {}, "mini-mail.com": {}, "easytrashmail.com": {},
	"emailtemporario.com.br": {}, "tempmail.plus": {}, "mailto.plus": {},
}
