package knowledge

import "testing"

func TestRewriteIsGroundedGolden(t *testing.T) {
	t.Parallel()

	prior := QATurn{
		Question: "Acme_NDA.pdf 里责任上限是多少？",
		Answer:   "各方责任上限见第 8 条。",
	}
	evidence := []followUpLLMEvidence{
		{
			SourceName: "Acme_NDA.pdf",
			Excerpt:    "Each party’s liability under this NDA is limited to one million USD.",
		},
	}

	cases := []struct {
		name      string
		userQuery string
		rewrite   string
		want      bool
	}{
		{
			name:      "grounded_filename_and_liability",
			userQuery: "他们免费吗？",
			rewrite:   "Acme_NDA.pdf liability million",
			want:      true,
		},
		{
			name:      "grounded_excerpt_token",
			userQuery: "What about liability?",
			rewrite:   "What is the liability in Acme_NDA.pdf?",
			want:      true,
		},
		{
			name:      "industry_trivia",
			userQuery: "那市场呢？",
			rewrite:   "What is a typical SaaS EBITDA multiple?",
			want:      false,
		},
		{
			name:      "competitor_invent",
			userQuery: "他们怎么样？",
			rewrite:   "How does Salesforce structure indemnity for Acme_NDA.pdf?",
			want:      false,
		},
		{
			name:      "pure_out_of_room",
			userQuery: "还有呢？",
			rewrite:   "Compare NVCA model docs for market-standard earnouts",
			want:      false,
		},
		{
			name:      "invented_cloud_credits",
			userQuery: "这个有吗？",
			rewrite:   "Does Acme_NDA.pdf include Google Cloud credits?",
			want:      false,
		},
	}

	for _, tc := range cases {
		got := rewriteIsGrounded(tc.rewrite, tc.userQuery, prior, SessionState{}, evidence)
		if got != tc.want {
			t.Errorf("%s: grounded=%v want %v (rewrite=%q)", tc.name, got, tc.want, tc.rewrite)
		}
	}
}

func TestRewriteIsGroundedFailsClosedWithoutCorpus(t *testing.T) {
	t.Parallel()
	if rewriteIsGrounded("anything searchable", "还有呢？", QATurn{}, SessionState{}, nil) {
		t.Fatal("empty prior/evidence must fail closed")
	}
}
