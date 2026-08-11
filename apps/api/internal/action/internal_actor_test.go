package action

import "testing"

func TestMemberEmailSetContains(t *testing.T) {
	set := NewMemberEmailSet([]string{" Owner@Acme.com ", "", "ops@acme.com"})
	if !set.Contains("owner@acme.com") {
		t.Fatal("expected owner match")
	}
	if !set.Contains("OPS@acme.com") {
		t.Fatal("expected case-insensitive match")
	}
	if set.Contains("lp@vc.com") {
		t.Fatal("external must not match")
	}
	if set.Contains("") {
		t.Fatal("empty email is not internal")
	}
	if (MemberEmailSet(nil)).Contains("owner@acme.com") {
		t.Fatal("nil set never matches")
	}
}

func TestSkipVisitorAttributedActor(t *testing.T) {
	set := NewMemberEmailSet([]string{"owner@acme.com"})

	if SkipVisitorAttributedActor(set, false, "") {
		t.Fatal("NULL email cannot prove internal — keep")
	}
	if SkipVisitorAttributedActor(set, true, "") {
		t.Fatal("empty attributed email is anonymous external — keep")
	}
	if SkipVisitorAttributedActor(set, true, "  ") {
		t.Fatal("whitespace-only is anonymous external — keep")
	}
	if !SkipVisitorAttributedActor(set, true, "Owner@Acme.com") {
		t.Fatal("member email must skip")
	}
	if SkipVisitorAttributedActor(set, true, "lp@vc.com") {
		t.Fatal("external email must not skip")
	}
}
