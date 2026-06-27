package dealroom

// FolderTemplate describes a folder within a room template.
type FolderTemplate struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

// Template describes a deal room template exposed to clients.
type Template struct {
	ID                     string           `json:"id"`
	Name                   string           `json:"name"`
	Description            string           `json:"description"`
	Scenario               string           `json:"scenario"`
	FolderStructure        []FolderTemplate `json:"folderStructure"`
	RecommendedFiles       []string         `json:"recommendedFiles"`
	DefaultPermissionLevel string           `json:"defaultPermissionLevel"`
	NDAEnabled             bool             `json:"ndaEnabled"`
}

var roomTemplates = []Template{
	{
		ID:                     "tmpl_seed",
		Name:                   "Seed Round Due Diligence",
		Description:            "Standard due diligence room for seed investors, including deck, financial model, team and legal documents.",
		Scenario:               "seed",
		DefaultPermissionLevel: "medium",
		NDAEnabled:             false,
		RecommendedFiles:       []string{"Pitch Deck.pdf", "Financial Model.xlsx", "Cap Table"},
		FolderStructure: []FolderTemplate{
			{Path: "/01-pitch-deck", Name: "01 Pitch Deck", Description: "Latest fundraising deck", SortOrder: 0},
			{Path: "/02-financials", Name: "02 Financials", Description: "Historical financials, projections and key assumptions", SortOrder: 1},
			{Path: "/03-team", Name: "03 Team", Description: "Founder resumes, org chart and hiring plan", SortOrder: 2},
			{Path: "/04-product", Name: "04 Product", Description: "Product demo, roadmap and technical architecture", SortOrder: 3},
			{Path: "/05-legal", Name: "05 Legal", Description: "Incorporation, shareholder agreement and option plan", SortOrder: 4},
		},
	},
	{
		ID:                     "tmpl_series_a",
		Name:                   "Series A Data Room",
		Description:            "In-depth due diligence for Series A firms, emphasizing product data, growth metrics and financial transparency.",
		Scenario:               "series-a",
		DefaultPermissionLevel: "high",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Investor Deck.pdf", "Metrics Dashboard", "ARR Waterfall"},
		FolderStructure: []FolderTemplate{
			{Path: "/01-executive-summary", Name: "01 Executive Summary", SortOrder: 0},
			{Path: "/02-growth-metrics", Name: "02 Growth Metrics", SortOrder: 1},
			{Path: "/03-financials-projections", Name: "03 Financials & Projections", SortOrder: 2},
			{Path: "/04-customer-proof", Name: "04 Customer Proof", SortOrder: 3},
			{Path: "/05-product-tech", Name: "05 Product & Tech", SortOrder: 4},
			{Path: "/06-legal-compliance", Name: "06 Legal & Compliance", SortOrder: 5},
		},
	},
	{
		ID:                     "tmpl_lp_update",
		Name:                   "LP Quarterly Update",
		Description:            "Quarterly performance, distribution and operations report for LPs, with bulk distribution and access tracking.",
		Scenario:               "lp-update",
		DefaultPermissionLevel: "low",
		NDAEnabled:             false,
		RecommendedFiles:       []string{"GP Letter.pdf", "LP Report.xlsx"},
		FolderStructure: []FolderTemplate{
			{Path: "/01-gp-letter", Name: "01 GP Letter", SortOrder: 0},
			{Path: "/02-performance-irr", Name: "02 Performance & IRR", SortOrder: 1},
			{Path: "/03-portfolio-updates", Name: "03 Portfolio Updates", SortOrder: 2},
			{Path: "/04-distributions-capital-calls", Name: "04 Distributions & Capital Calls", SortOrder: 3},
			{Path: "/05-governance-aml-kyc", Name: "05 Governance & AML/KYC", SortOrder: 4},
		},
	},
	{
		ID:                     "tmpl_sales_proposal",
		Name:                   "Enterprise Sales Proposal",
		Description:            "Proposal data room for enterprise procurement committees, including proposal, case studies, security and implementation plan.",
		Scenario:               "sales-proposal",
		DefaultPermissionLevel: "medium",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Proposal.pdf", "Security Whitepaper", "Implementation Plan"},
		FolderStructure: []FolderTemplate{
			{Path: "/01-proposal", Name: "01 Proposal", SortOrder: 0},
			{Path: "/02-roi-business-case", Name: "02 ROI & Business Case", SortOrder: 1},
			{Path: "/03-case-studies", Name: "03 Case Studies", SortOrder: 2},
			{Path: "/04-security-compliance", Name: "04 Security & Compliance", SortOrder: 3},
			{Path: "/05-implementation-plan", Name: "05 Implementation Plan", SortOrder: 4},
		},
	},
}

func templateFolders(templateType string) []FolderTemplate {
	for _, t := range roomTemplates {
		if t.ID == templateType || t.Scenario == templateType {
			return t.FolderStructure
		}
	}
	return nil
}
