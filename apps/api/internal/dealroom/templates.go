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
		ID:                     "tmpl_startup_fundraising",
		Name:                   "Startup Fundraising",
		Description:            "Data room for startup fundraising, covering corporate memo, financials, legal, GTM, product roadmap and pitch deck.",
		Scenario:               "startup-fundraising",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Pitch Deck.pdf", "Financial Model.xlsx", "Investment Memo.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/corporate-or-investment-memo", Name: "Corporate or Investment Memo", Description: "Investment memo or corporate overview", SortOrder: 0},
			{Path: "/corporate-documents", Name: "Corporate Documents", Description: "Certificate of incorporation, bylaws, cap table", SortOrder: 1},
			{Path: "/financial-forecast-and-actuals", Name: "Financial Forecast and Actuals", Description: "Historical financials, projections and assumptions", SortOrder: 2},
			{Path: "/legal-and-tax-documents", Name: "Legal and Tax Documents", Description: "Legal agreements, tax filings and compliance", SortOrder: 3},
			{Path: "/go-to-market-and-marketing-strategy", Name: "Go-to-Market and Marketing Strategy", Description: "GTM plan, marketing strategy and sales materials", SortOrder: 4},
			{Path: "/product-roadmap", Name: "Product Roadmap", Description: "Product roadmap and technical architecture", SortOrder: 5},
			{Path: "/pitch-deck", Name: "Pitch Deck", Description: "Latest fundraising deck", SortOrder: 6},
		},
	},
	{
		ID:                     "tmpl_raising_first_fund",
		Name:                   "Raising First Fund",
		Description:            "Data room for first-time fund managers, covering team, track record, fund model, legal and portfolio.",
		Scenario:               "raising-first-fund",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Fund Pitch Deck.pdf", "Track Record.xlsx", "PPM.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/introduction", Name: "Introduction", Description: "Fund introduction and thesis", SortOrder: 0},
			{Path: "/team", Name: "Team", Description: "GP and team backgrounds", SortOrder: 1},
			{Path: "/track-record", Name: "Track Record", Description: "Investment track record and case studies", SortOrder: 2},
			{Path: "/fund-model", Name: "Fund Model", Description: "Fund size, strategy, economics and terms", SortOrder: 3},
			{Path: "/legal", Name: "Legal", Description: "LPA, PPM, subscription documents", SortOrder: 4},
			{Path: "/portfolio", Name: "Portfolio", Description: "Portfolio construction and pipeline", SortOrder: 5},
		},
	},
	{
		ID:                     "tmpl_ma_acquisition",
		Name:                   "M&A Acquisition",
		Description:            "Due diligence data room for mergers and acquisitions, covering corporate, financial, legal, IP, contracts, HR, tax, assets and insurance.",
		Scenario:               "ma-acquisition",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Executive Summary.pdf", "Financial Statements.xlsx", "Cap Table"},
		FolderStructure: []FolderTemplate{
			{Path: "/executive-summary", Name: "Executive Summary", Description: "Transaction overview and key highlights", SortOrder: 0},
			{Path: "/corporate-structure-and-governance", Name: "Corporate Structure & Governance", Description: "Org chart, governance documents and board minutes", SortOrder: 1},
			{Path: "/financial-information", Name: "Financial Information", Description: "Financial statements, reports and projections", SortOrder: 2},
			{Path: "/legal-and-compliance", Name: "Legal & Compliance", Description: "Regulatory, compliance and legal matters", SortOrder: 3},
			{Path: "/intellectual-property", Name: "Intellectual Property", Description: "IP portfolio, patents, trademarks and licenses", SortOrder: 4},
			{Path: "/contracts-and-agreements", Name: "Contracts & Agreements", Description: "Material contracts and customer agreements", SortOrder: 5},
			{Path: "/human-resources", Name: "Human Resources", Description: "Employee information, benefits and policies", SortOrder: 6},
			{Path: "/tax-documents", Name: "Tax Documents", Description: "Tax returns, filings and transfer pricing", SortOrder: 7},
			{Path: "/assets-and-liabilities", Name: "Assets & Liabilities", Description: "Fixed assets, debt schedule and contingent liabilities", SortOrder: 8},
			{Path: "/insurance", Name: "Insurance", Description: "Insurance policies and coverage", SortOrder: 9},
		},
	},
	{
		ID:                     "tmpl_series_a_plus",
		Name:                   "Series A+",
		Description:            "Growth-stage due diligence data room for Series A and beyond, covering investment memo, financials, cap table, product, market, team, legal and competitive analysis.",
		Scenario:               "series-a-plus",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Investment Memorandum.pdf", "Financial Model.xlsx", "Cap Table"},
		FolderStructure: []FolderTemplate{
			{Path: "/investment-memorandum", Name: "Investment Memorandum", Description: "Investment thesis and company overview", SortOrder: 0},
			{Path: "/financial-information", Name: "Financial Information", Description: "Financial statements, metrics and projections", SortOrder: 1},
			{Path: "/corporate-documents", Name: "Corporate Documents", Description: "Incorporation, bylaws and governance", SortOrder: 2},
			{Path: "/cap-table-and-term-sheets", Name: "Cap Table & Term Sheets", Description: "Ownership structure and term sheets", SortOrder: 3},
			{Path: "/product-and-technology", Name: "Product & Technology", Description: "Product roadmap, tech stack and demos", SortOrder: 4},
			{Path: "/market-and-traction", Name: "Market & Traction", Description: "Market size, traction and growth metrics", SortOrder: 5},
			{Path: "/team-and-organization", Name: "Team & Organization", Description: "Team bios, org chart and hiring plan", SortOrder: 6},
			{Path: "/legal-and-ip", Name: "Legal & IP", Description: "Legal documents and intellectual property", SortOrder: 7},
			{Path: "/competitive-analysis", Name: "Competitive Analysis", Description: "Competitive landscape and positioning", SortOrder: 8},
			{Path: "/use-of-funds", Name: "Use of Funds", Description: "Allocation and use of proceeds", SortOrder: 9},
		},
	},
	{
		ID:                     "tmpl_real_estate_transaction",
		Name:                   "Real Estate Transaction",
		Description:            "Data room for real estate transactions, covering property info, title, legal, financials, leases, surveys, environmental, inspections, management and insurance.",
		Scenario:               "real-estate-transaction",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Property Information.pdf", "Lease Schedule.xlsx", "Title Report.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/property-information", Name: "Property Information", Description: "Property details, photos and specifications", SortOrder: 0},
			{Path: "/title-and-ownership", Name: "Title & Ownership", Description: "Title report and ownership documents", SortOrder: 1},
			{Path: "/legal-documents", Name: "Legal Documents", Description: "Purchase agreement and legal documents", SortOrder: 2},
			{Path: "/financial-information", Name: "Financial Information", Description: "Financials, rent roll and operating statements", SortOrder: 3},
			{Path: "/leases-and-tenancies", Name: "Leases & Tenancies", Description: "Lease agreements and tenant information", SortOrder: 4},
			{Path: "/property-surveys-and-plans", Name: "Property Surveys & Plans", Description: "Surveys, floor plans and site plans", SortOrder: 5},
			{Path: "/environmental-reports", Name: "Environmental Reports", Description: "Environmental assessments and reports", SortOrder: 6},
			{Path: "/building-inspections", Name: "Building Inspections", Description: "Inspection reports and certificates", SortOrder: 7},
			{Path: "/property-management", Name: "Property Management", Description: "Management agreements and service contracts", SortOrder: 8},
			{Path: "/insurance-and-warranties", Name: "Insurance & Warranties", Description: "Insurance policies and warranties", SortOrder: 9},
		},
	},
	{
		ID:                     "tmpl_fund_management",
		Name:                   "Fund Management",
		Description:            "Data room for ongoing fund management, covering fund documents, LP relations, reporting, compliance, investments, monitoring, operations and communications.",
		Scenario:               "fund-management",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"LPA.pdf", "Quarterly Report.xlsx", "Capital Account Statement.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/fund-documents", Name: "Fund Documents", Description: "LPA, PPM and offering documents", SortOrder: 0},
			{Path: "/lp-relations", Name: "LP Relations", Description: "LP agreements, KYC and communication", SortOrder: 1},
			{Path: "/financial-reporting", Name: "Financial Reporting", Description: "Financial statements and capital accounts", SortOrder: 2},
			{Path: "/compliance-and-legal", Name: "Compliance & Legal", Description: "Compliance, regulatory and legal matters", SortOrder: 3},
			{Path: "/investment-activities", Name: "Investment Activities", Description: "Investment memos and approvals", SortOrder: 4},
			{Path: "/portfolio-monitoring", Name: "Portfolio Monitoring", Description: "Portfolio company updates and metrics", SortOrder: 5},
			{Path: "/operations", Name: "Operations", Description: "Fund operations and administration", SortOrder: 6},
			{Path: "/investor-communications", Name: "Investor Communications", Description: "GP letters and investor notices", SortOrder: 7},
		},
	},
	{
		ID:                     "tmpl_portfolio_management",
		Name:                   "Portfolio Management",
		Description:            "Data room for portfolio company management, covering overview, companies, performance, governance, support, deals, exits and monitoring.",
		Scenario:               "portfolio-management",
		DefaultPermissionLevel: "confidential",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Portfolio Overview.pdf", "Financial Dashboard.xlsx", "Board Deck.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/portfolio-overview", Name: "Portfolio Overview", Description: "Portfolio summary and strategy", SortOrder: 0},
			{Path: "/portfolio-companies", Name: "Portfolio Companies", Description: "Company profiles and investment memos", SortOrder: 1},
			{Path: "/financial-performance", Name: "Financial Performance", Description: "Financial results and KPIs", SortOrder: 2},
			{Path: "/board-and-governance", Name: "Board & Governance", Description: "Board materials and governance documents", SortOrder: 3},
			{Path: "/operational-support", Name: "Operational Support", Description: "Operational support and resources", SortOrder: 4},
			{Path: "/deal-documents", Name: "Deal Documents", Description: "Investment documents and agreements", SortOrder: 5},
			{Path: "/follow-on-and-exits", Name: "Follow-on & Exits", Description: "Follow-on financings and exit materials", SortOrder: 6},
			{Path: "/portfolio-monitoring", Name: "Portfolio Monitoring", Description: "Ongoing monitoring and reporting", SortOrder: 7},
		},
	},
	{
		ID:                     "tmpl_project_management",
		Name:                   "Project Management",
		Description:            "Data room for project management, covering overview, requirements, planning, team, execution, communication, risk, quality, documentation and closure.",
		Scenario:               "project-management",
		DefaultPermissionLevel: "standard",
		NDAEnabled:             false,
		RecommendedFiles:       []string{"Project Charter.pdf", "Project Plan.xlsx", "Requirements Doc.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/project-overview", Name: "Project Overview", Description: "Project charter, objectives and scope", SortOrder: 0},
			{Path: "/requirements-and-specifications", Name: "Requirements & Specifications", Description: "Requirements and technical specifications", SortOrder: 1},
			{Path: "/project-planning", Name: "Project Planning", Description: "Schedule, budget and resource plan", SortOrder: 2},
			{Path: "/team-and-stakeholders", Name: "Team & Stakeholders", Description: "Team roster and stakeholder register", SortOrder: 3},
			{Path: "/project-execution", Name: "Project Execution", Description: "Status reports and deliverables", SortOrder: 4},
			{Path: "/communication", Name: "Communication", Description: "Communication plan and meeting notes", SortOrder: 5},
			{Path: "/risk-and-issues", Name: "Risk & Issues", Description: "Risk register and issue log", SortOrder: 6},
			{Path: "/quality-and-testing", Name: "Quality & Testing", Description: "Quality plan and test results", SortOrder: 7},
			{Path: "/documentation", Name: "Documentation", Description: "Project documentation and reference materials", SortOrder: 8},
			{Path: "/project-closure", Name: "Project Closure", Description: "Closure report and lessons learned", SortOrder: 9},
		},
	},
	{
		ID:                     "tmpl_sales_dataroom",
		Name:                   "Sales Data Room",
		Description:            "Data room for enterprise sales, covering sales materials, proposals, contracts, product info, customer references and security compliance.",
		Scenario:               "sales-dataroom",
		DefaultPermissionLevel: "standard",
		NDAEnabled:             true,
		RecommendedFiles:       []string{"Proposal.pdf", "Security Whitepaper.pdf", "Case Studies.pdf"},
		FolderStructure: []FolderTemplate{
			{Path: "/sales-materials", Name: "Sales Materials", Description: "Brochures, one-pagers and sales decks", SortOrder: 0},
			{Path: "/proposals-and-quotes", Name: "Proposals & Quotes", Description: "Proposals, quotes and pricing", SortOrder: 1},
			{Path: "/contracts-and-agreements", Name: "Contracts & Agreements", Description: "MSA, order forms and agreements", SortOrder: 2},
			{Path: "/product-information", Name: "Product Information", Description: "Product docs, datasheets and demos", SortOrder: 3},
			{Path: "/customer-references", Name: "Customer References", Description: "Case studies and reference customers", SortOrder: 4},
			{Path: "/security-and-compliance", Name: "Security & Compliance", Description: "Security, compliance and certifications", SortOrder: 5},
		},
	},
	{
		ID:                     "tmpl_custom",
		Name:                   "Completely Custom",
		Description:            "Start with an empty room and add your own folders.",
		Scenario:               "custom",
		DefaultPermissionLevel: "standard",
		NDAEnabled:             false,
		RecommendedFiles:       []string{},
		FolderStructure:        []FolderTemplate{},
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
