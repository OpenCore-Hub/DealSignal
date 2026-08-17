package emailid

import "testing"

func TestCanonical(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"  Foo.Bar+deal@Gmail.COM ": "foobar@gmail.com",
		"ab@googlemail.com":         "ab@gmail.com",
		"a.b@googlemail.com":        "ab@gmail.com",
		"user+tag@outlook.com":      "user@outlook.com",
		"user+tag@acme.com":         "user@acme.com",
		"User@Acme.COM":             "user@acme.com",
	}
	for in, want := range cases {
		if got := Canonical(in); got != want {
			t.Fatalf("Canonical(%q)=%q want %q", in, got, want)
		}
	}
}

func TestIsDisposable(t *testing.T) {
	t.Parallel()
	if !IsDisposable("a+1@mailinator.com") {
		t.Fatal("mailinator must be disposable")
	}
	if !IsDisposable("x@yopmail.com") {
		t.Fatal("yopmail must be disposable")
	}
	if IsDisposable("founder@acme.com") {
		t.Fatal("corporate inbox must not be disposable")
	}
	if IsDisposable("user@gmail.com") {
		t.Fatal("gmail must not be disposable")
	}
	if !IsDisposable("x@foo.mailinator.com") {
		t.Fatal("mailinator subdomain must be disposable")
	}
}

func TestKeys(t *testing.T) {
	t.Parallel()
	got := Keys("  Jane.Doe+vdr@Gmail.COM ")
	if len(got) != 2 || got[0] != "jane.doe+vdr@gmail.com" || got[1] != "janedoe@gmail.com" {
		t.Fatalf("Keys=%v", got)
	}
	if got := Keys("user@acme.com"); len(got) != 1 || got[0] != "user@acme.com" {
		t.Fatalf("plain Keys=%v", got)
	}
	if Keys("  ") != nil {
		t.Fatal("empty")
	}
}

func TestSameMailbox(t *testing.T) {
	t.Parallel()
	if !SameMailbox("Jane.Doe+vdr@Gmail.com", "janedoe@gmail.com") {
		t.Fatal("gmail plus-tag and dots must be one mailbox")
	}
	if SameMailbox("a@acme.com", "b@acme.com") {
		t.Fatal("distinct local parts must not match")
	}
}
