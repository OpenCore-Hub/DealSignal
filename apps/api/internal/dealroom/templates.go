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
			{Path: "/01-corporate-or-investment-memo", Name: "01 Corporate or Investment Memo", Description: "Investment memo or corporate overview", SortOrder: 0},
			{Path: "/02-corporate-documents", Name: "02 Corporate Documents", Description: "Certificate of incorporation, bylaws, cap table", SortOrder: 1},
			{Path: "/03-financial-forecast-and-actuals", Name: "03 Financial Forecast and Actuals", Description: "Historical financials, projections and assumptions", SortOrder: 2},
			{Path: "/04-legal-and-tax-documents", Name: "04 Legal and Tax Documents", Description: "Legal agreements, tax filings and compliance", SortOrder: 3},
			{Path: "/05-go-to-market-and-marketing-strategy", Name: "05 Go-to-Market and Marketing Strategy", Description: "GTM plan, marketing strategy and sales materials", SortOrder: 4},
			{Path: "/06-product-roadmap", Name: "06 Product Roadmap", Description: "Product roadmap and technical architecture", SortOrder: 5},
			{Path: "/07-pitch-deck", Name: "07 Pitch Deck", Description: "Latest fundraising deck", SortOrder: 6},
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
			{Path: "/01-introduction", Name: "01 Introduction", Description: "Fund introduction and thesis", SortOrder: 0},
			{Path: "/02-team", Name: "02 Team", Description: "GP and team backgrounds", SortOrder: 1},
			{Path: "/03-track-record", Name: "03 Track Record", Description: "Investment track record and case studies", SortOrder: 2},
			{Path: "/04-fund-model", Name: "04 Fund Model", Description: "Fund size, strategy, economics and terms", SortOrder: 3},
			{Path: "/05-legal", Name: "05 Legal", Description: "LPA, PPM, subscription documents", SortOrder: 4},
			{Path: "/06-portfolio", Name: "06 Portfolio", Description: "Portfolio construction and pipeline", SortOrder: 5},
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
			{Path: "/01-executive-summary", Name: "01 Executive Summary", Description: "Transaction overview and key highlights", SortOrder: 0},
			{Path: "/02-corporate-structure-and-governance", Name: "02 Corporate Structure & Governance", Description: "Org chart, governance documents and board minutes", SortOrder: 1},
			{Path: "/03-financial-information", Name: "03 Financial Information", Description: "Financial statements, reports and projections", SortOrder: 2},
			{Path: "/04-legal-and-compliance", Name: "04 Legal & Compliance", Description: "Regulatory, compliance and legal matters", SortOrder: 3},
			{Path: "/05-intellectual-property", Name: "05 Intellectual Property", Description: "IP portfolio, patents, trademarks and licenses", SortOrder: 4},
			{Path: "/06-contracts-and-agreements", Name: "06 Contracts & Agreements", Description: "Material contracts and customer agreements", SortOrder: 5},
			{Path: "/07-human-resources", Name: "07 Human Resources", Description: "Employee information, benefits and policies", SortOrder: 6},
			{Path: "/08-tax-documents", Name: "08 Tax Documents", Description: "Tax returns, filings and transfer pricing", SortOrder: 7},
			{Path: "/09-assets-and-liabilities", Name: "09 Assets & Liabilities", Description: "Fixed assets, debt schedule and contingent liabilities", SortOrder: 8},
			{Path: "/10-insurance", Name: "10 Insurance", Description: "Insurance policies and coverage", SortOrder: 9},
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
			{Path: "/01-investment-memorandum", Name: "01 Investment Memorandum", Description: "Investment thesis and company overview", SortOrder: 0},
			{Path: "/02-financial-information", Name: "02 Financial Information", Description: "Financial statements, metrics and projections", SortOrder: 1},
			{Path: "/03-corporate-documents", Name: "03 Corporate Documents", Description: "Incorporation, bylaws and governance", SortOrder: 2},
			{Path: "/04-cap-table-and-term-sheets", Name: "04 Cap Table & Term Sheets", Description: "Ownership structure and term sheets", SortOrder: 3},
			{Path: "/05-product-and-technology", Name: "05 Product & Technology", Description: "Product roadmap, tech stack and demos", SortOrder: 4},
			{Path: "/06-market-and-traction", Name: "06 Market & Traction", Description: "Market size, traction and growth metrics", SortOrder: 5},
			{Path: "/07-team-and-organization", Name: "07 Team & Organization", Description: "Team bios, org chart and hiring plan", SortOrder: 6},
			{Path: "/08-legal-and-ip", Name: "08 Legal & IP", Description: "Legal documents and intellectual property", SortOrder: 7},
			{Path: "/09-competitive-analysis", Name: "09 Competitive Analysis", Description: "Competitive landscape and positioning", SortOrder: 8},
			{Path: "/10-use-of-funds", Name: "10 Use of Funds", Description: "Allocation and use of proceeds", SortOrder: 9},
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
			{Path: "/01-property-information", Name: "01 Property Information", Description: "Property details, photos and specifications", SortOrder: 0},
			{Path: "/02-title-and-ownership", Name: "02 Title & Ownership", Description: "Title report and ownership documents", SortOrder: 1},
			{Path: "/03-legal-documents", Name: "03 Legal Documents", Description: "Purchase agreement and legal documents", SortOrder: 2},
			{Path: "/04-financial-information", Name: "04 Financial Information", Description: "Financials, rent roll and operating statements", SortOrder: 3},
			{Path: "/05-leases-and-tenancies", Name: "05 Leases & Tenancies", Description: "Lease agreements and tenant information", SortOrder: 4},
			{Path: "/06-property-surveys-and-plans", Name: "06 Property Surveys & Plans", Description: "Surveys, floor plans and site plans", SortOrder: 5},
			{Path: "/07-environmental-reports", Name: "07 Environmental Reports", Description: "Environmental assessments and reports", SortOrder: 6},
			{Path: "/08-building-inspections", Name: "08 Building Inspections", Description: "Inspection reports and certificates", SortOrder: 7},
			{Path: "/09-property-management", Name: "09 Property Management", Description: "Management agreements and service contracts", SortOrder: 8},
			{Path: "/10-insurance-and-warranties", Name: "10 Insurance & Warranties", Description: "Insurance policies and warranties", SortOrder: 9},
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
			{Path: "/01-fund-documents", Name: "01 Fund Documents", Description: "LPA, PPM and offering documents", SortOrder: 0},
			{Path: "/02-lp-relations", Name: "02 LP Relations", Description: "LP agreements, KYC and communication", SortOrder: 1},
			{Path: "/03-financial-reporting", Name: "03 Financial Reporting", Description: "Financial statements and capital accounts", SortOrder: 2},
			{Path: "/04-compliance-and-legal", Name: "04 Compliance & Legal", Description: "Compliance, regulatory and legal matters", SortOrder: 3},
			{Path: "/05-investment-activities", Name: "05 Investment Activities", Description: "Investment memos and approvals", SortOrder: 4},
			{Path: "/06-portfolio-monitoring", Name: "06 Portfolio Monitoring", Description: "Portfolio company updates and metrics", SortOrder: 5},
			{Path: "/07-operations", Name: "07 Operations", Description: "Fund operations and administration", SortOrder: 6},
			{Path: "/08-investor-communications", Name: "08 Investor Communications", Description: "GP letters and investor notices", SortOrder: 7},
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
			{Path: "/01-portfolio-overview", Name: "01 Portfolio Overview", Description: "Portfolio summary and strategy", SortOrder: 0},
			{Path: "/02-portfolio-companies", Name: "02 Portfolio Companies", Description: "Company profiles and investment memos", SortOrder: 1},
			{Path: "/03-financial-performance", Name: "03 Financial Performance", Description: "Financial results and KPIs", SortOrder: 2},
			{Path: "/04-board-and-governance", Name: "04 Board & Governance", Description: "Board materials and governance documents", SortOrder: 3},
			{Path: "/05-operational-support", Name: "05 Operational Support", Description: "Operational support and resources", SortOrder: 4},
			{Path: "/06-deal-documents", Name: "06 Deal Documents", Description: "Investment documents and agreements", SortOrder: 5},
			{Path: "/07-follow-on-and-exits", Name: "07 Follow-on & Exits", Description: "Follow-on financings and exit materials", SortOrder: 6},
			{Path: "/08-portfolio-monitoring", Name: "08 Portfolio Monitoring", Description: "Ongoing monitoring and reporting", SortOrder: 7},
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
			{Path: "/01-project-overview", Name: "01 Project Overview", Description: "Project charter, objectives and scope", SortOrder: 0},
			{Path: "/02-requirements-and-specifications", Name: "02 Requirements & Specifications", Description: "Requirements and technical specifications", SortOrder: 1},
			{Path: "/03-project-planning", Name: "03 Project Planning", Description: "Schedule, budget and resource plan", SortOrder: 2},
			{Path: "/04-team-and-stakeholders", Name: "04 Team & Stakeholders", Description: "Team roster and stakeholder register", SortOrder: 3},
			{Path: "/05-project-execution", Name: "05 Project Execution", Description: "Status reports and deliverables", SortOrder: 4},
			{Path: "/06-communication", Name: "06 Communication", Description: "Communication plan and meeting notes", SortOrder: 5},
			{Path: "/07-risk-and-issues", Name: "07 Risk & Issues", Description: "Risk register and issue log", SortOrder: 6},
			{Path: "/08-quality-and-testing", Name: "08 Quality & Testing", Description: "Quality plan and test results", SortOrder: 7},
			{Path: "/09-documentation", Name: "09 Documentation", Description: "Project documentation and knowledge base", SortOrder: 8},
			{Path: "/10-project-closure", Name: "10 Project Closure", Description: "Closure report and lessons learned", SortOrder: 9},
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
			{Path: "/01-sales-materials", Name: "01 Sales Materials", Description: "Brochures, one-pagers and sales decks", SortOrder: 0},
			{Path: "/02-proposals-and-quotes", Name: "02 Proposals & Quotes", Description: "Proposals, quotes and pricing", SortOrder: 1},
			{Path: "/03-contracts-and-agreements", Name: "03 Contracts & Agreements", Description: "MSA, order forms and agreements", SortOrder: 2},
			{Path: "/04-product-information", Name: "04 Product Information", Description: "Product docs, datasheets and demos", SortOrder: 3},
			{Path: "/05-customer-references", Name: "05 Customer References", Description: "Case studies and reference customers", SortOrder: 4},
			{Path: "/06-security-and-compliance", Name: "06 Security & Compliance", Description: "Security, compliance and certifications", SortOrder: 5},
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
