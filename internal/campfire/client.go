package campfire

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/arr-tracker/internal/models"
)

const (
	baseURL  = "https://api.meetcampfire.com"
	pageSize = 100
)

// Client is a Campfire REST API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a new Campfire client.
func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchAllContracts retrieves all contracts from Campfire, paginating automatically.
// If sinceTime is non-zero, only contracts modified after that time are returned (incremental sync).
func (c *Client) FetchAllContracts(sinceTime *time.Time) ([]models.CampfireContract, error) {
	var all []models.CampfireContract
	offset := 0

	for {
		batch, total, err := c.fetchContractsPage(offset, sinceTime)
		if err != nil {
			return nil, fmt.Errorf("fetching page at offset %d: %w", offset, err)
		}

		all = append(all, batch...)
		offset += len(batch)

		if offset >= total || len(batch) == 0 {
			break
		}
	}

	return all, nil
}

func (c *Client) fetchContractsPage(offset int, sinceTime *time.Time) ([]models.CampfireContract, int, error) {
	endpoint := fmt.Sprintf("%s/rr/api/v1/contracts", baseURL)

	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", pageSize))
	params.Set("offset", fmt.Sprintf("%d", offset))
	if sinceTime != nil {
		params.Set("last_modified_at__gte", sinceTime.UTC().Format(time.RFC3339))
	}

	req, err := http.NewRequest("GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("campfire API returned status %d", resp.StatusCode)
	}

	var result models.CampfireResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("decoding response: %w", err)
	}

	return result.Results, result.Count, nil
}

// NormalizeContract converts a raw Campfire contract into a normalized Contract
// ready for database storage, including ARR calculations.
func NormalizeContract(c models.CampfireContract) (models.Contract, error) {
	now := time.Now().UTC()

	// Parse last_modified_at
	lastMod, err := time.Parse(time.RFC3339, c.LastModifiedAt)
	if err != nil {
		// Fall back gracefully
		lastMod = now
	}

	// Parse contract dates for duration calculation
	startDate, _ := time.Parse("2006-01-02", c.ContractStartDate)
	endDate, _ := time.Parse("2006-01-02", c.ContractEndDate)

	var contractMonths float64
	if !startDate.IsZero() && !endDate.IsZero() && endDate.After(startDate) {
		// More precise: count calendar months
		years := endDate.Year() - startDate.Year()
		months := int(endDate.Month()) - int(startDate.Month())
		days := endDate.Day() - startDate.Day()
		contractMonths = float64(years*12+months) + float64(days)/30.0
		contractMonths = math.Round(contractMonths*100) / 100
	}

	// ARR methodology (Coder):
	//   ARR = (total_contract_value / contract_months) * 12
	//
	//   This correctly annualizes TCV across the full contract duration, consistent
	//   with the 85/15 SSP allocation used in ASC 606 revenue recognition.
	//   MRR is NOT used because it reflects only the support component post-allocation.
	//
	//   USD normalization uses exchange_rate locked at contract signing (spot-rate
	//   methodology), so non-USD contracts are not re-measured at current rates.
	var arr float64
	if contractMonths > 0 {
		arr = math.Round((c.TotalContractValue/contractMonths)*12*100) / 100
	}

	exchangeRate := c.ExchangeRate
	if exchangeRate == 0 {
		exchangeRate = 1.0
	}
	arrUSD := math.Round(arr*exchangeRate*100) / 100

	return models.Contract{
		CampfireID:         c.ID,
		ClientName:         c.ClientName,
		DealName:           c.DealName,
		DealID:             c.DealID,
		Status:             c.Status,
		Currency:           c.Currency,
		BillingFrequency:   c.BillingFrequency,
		ContractStartDate:  c.ContractStartDate,
		ContractEndDate:    c.ContractEndDate,
		ClosedDate:         c.ClosedDate,
		TotalContractValue: c.TotalContractValue,
		TotalBilled:        c.TotalBilled,
		TotalMRR:           c.TotalMRR,
		ARR:                arr,
		ARRUSD:             arrUSD,
		ExchangeRate:       exchangeRate,
		ContractMonths:     contractMonths,
		IsEvergreen:        c.IsEvergreen,
		OpportunityID:      c.CustomFieldOpportunityID,
		LastModifiedAt:     lastMod,
		SyncedAt:           now,
	}, nil
}
