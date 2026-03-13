package models

import "time"

// CampfireContract mirrors the Campfire /rr/api/v1/contracts response.
type CampfireContract struct {
	ID                int     `json:"id"`
	ClientName        string  `json:"client_name"`
	DealName          string  `json:"deal_name"`
	DealID            string  `json:"deal_id"`
	Status            string  `json:"status"`
	Source            string  `json:"source"`
	Currency          string  `json:"currency"`
	EntityCurrency    string  `json:"entity_currency"`
	BillingFrequency  string  `json:"billing_frequency"`
	ContractStartDate string  `json:"contract_start_date"` // "YYYY-MM-DD"
	ContractEndDate   string  `json:"contract_end_date"`
	ClosedDate        string  `json:"closed_date"`
	LastModifiedAt    string  `json:"last_modified_at"`
	TotalContractValue float64 `json:"total_contract_value"`
	TotalBilled       float64 `json:"total_billed"`
	TotalMRR          float64 `json:"total_mrr"`
	TotalRevenue      float64 `json:"total_revenue"`
	TotalDeferred     float64 `json:"total_deferred_revenue"`
	ExchangeRate      float64 `json:"exchange_rate"`
	ExchangeRateBook  float64 `json:"exchange_rate_book"`
	IsEvergreen       bool    `json:"is_evergreen"`

	// Custom fields populated by Coder's Campfire config
	CustomFieldARR           string `json:"custom_field_arr"`
	CustomFieldContractTerm  string `json:"custom_field_contract_term"`
	CustomFieldOpportunityID string `json:"custom_field_opportunity_id"`
}

// CampfireResponse is the paginated response wrapper from Campfire.
type CampfireResponse struct {
	Count   int                `json:"count"`
	Next    *string            `json:"next"`
	Results []CampfireContract `json:"results"`
}

// Contract is the normalized record stored in Supabase and returned by the API.
type Contract struct {
	ID                 int       `json:"id"`
	CampfireID         int       `json:"campfire_id"`
	ClientName         string    `json:"client_name"`
	DealName           string    `json:"deal_name"`
	DealID             string    `json:"deal_id"`
	Status             string    `json:"status"`
	Currency           string    `json:"currency"`
	BillingFrequency   string    `json:"billing_frequency"`
	ContractStartDate  string    `json:"contract_start_date"`
	ContractEndDate    string    `json:"contract_end_date"`
	ClosedDate         string    `json:"closed_date"`
	TotalContractValue float64   `json:"total_contract_value"`
	TotalBilled        float64   `json:"total_billed"`
	TotalMRR           float64   `json:"total_mrr"`
	// ARR = TotalMRR * 12, in contract currency
	ARR                float64   `json:"arr"`
	// ARR_USD uses exchange_rate locked at signing (your spot-rate methodology)
	ARRUSD             float64   `json:"arr_usd"`
	ExchangeRate       float64   `json:"exchange_rate"`
	ContractMonths     float64   `json:"contract_months"`
	IsEvergreen        bool      `json:"is_evergreen"`
	OpportunityID      string    `json:"opportunity_id"`
	LastModifiedAt     time.Time `json:"last_modified_at"`
	SyncedAt           time.Time `json:"synced_at"`
}

// Summary is the aggregated ARR view returned by /api/summary.
type Summary struct {
	TotalARRUSD       float64        `json:"total_arr_usd"`
	TotalMRRUSD       float64        `json:"total_mrr_usd"`
	ActiveContracts   int            `json:"active_contracts"`
	EvergreenContracts int           `json:"evergreen_contracts"`
	ByCurrency        []CurrencyARR  `json:"by_currency"`
	LastSyncedAt      *time.Time     `json:"last_synced_at"`
	ContractCount     int            `json:"contract_count"`
}

// CurrencyARR shows ARR breakdown by source currency.
type CurrencyARR struct {
	Currency string  `json:"currency"`
	ARR      float64 `json:"arr"`
	ARRUSD   float64 `json:"arr_usd"`
	Count    int     `json:"count"`
}

// SyncResult is returned after a sync operation.
type SyncResult struct {
	Upserted   int       `json:"upserted"`
	Total      int       `json:"total"`
	SyncedAt   time.Time `json:"synced_at"`
	Incremental bool     `json:"incremental"`
}
