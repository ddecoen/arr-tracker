package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/coder/arr-tracker/internal/models"
)

// DB wraps a Postgres connection.
type DB struct {
	conn *sql.DB
}

// New opens and validates a Postgres connection.
func New(connStr string) (*DB, error) {
	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}
	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(5 * time.Minute)
	return &DB{conn: conn}, nil
}

// Migrate creates the contracts table and sync_log if they don't exist.
func (db *DB) Migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS contracts (
			id                   SERIAL PRIMARY KEY,
			campfire_id          INTEGER UNIQUE NOT NULL,
			client_name          TEXT NOT NULL DEFAULT '',
			deal_name            TEXT NOT NULL DEFAULT '',
			deal_id              TEXT NOT NULL DEFAULT '',
			status               TEXT NOT NULL DEFAULT '',
			currency             TEXT NOT NULL DEFAULT 'USD',
			billing_frequency    TEXT NOT NULL DEFAULT '',
			contract_start_date  DATE,
			contract_end_date    DATE,
			closed_date          DATE,
			total_contract_value NUMERIC(18,2) NOT NULL DEFAULT 0,
			total_billed         NUMERIC(18,2) NOT NULL DEFAULT 0,
			total_mrr            NUMERIC(18,2) NOT NULL DEFAULT 0,
			arr                  NUMERIC(18,2) NOT NULL DEFAULT 0,
			arr_usd              NUMERIC(18,2) NOT NULL DEFAULT 0,
			exchange_rate        NUMERIC(12,6) NOT NULL DEFAULT 1,
			contract_months      NUMERIC(8,2)  NOT NULL DEFAULT 0,
			is_evergreen         BOOLEAN NOT NULL DEFAULT FALSE,
			opportunity_id       TEXT NOT NULL DEFAULT '',
			last_modified_at     TIMESTAMPTZ NOT NULL,
			synced_at            TIMESTAMPTZ NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_contracts_status    ON contracts(status);
		CREATE INDEX IF NOT EXISTS idx_contracts_arr_usd   ON contracts(arr_usd DESC);
		CREATE INDEX IF NOT EXISTS idx_contracts_synced_at ON contracts(synced_at DESC);

		CREATE TABLE IF NOT EXISTS sync_log (
			id           SERIAL PRIMARY KEY,
			synced_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			upserted     INTEGER NOT NULL DEFAULT 0,
			total        INTEGER NOT NULL DEFAULT 0,
			incremental  BOOLEAN NOT NULL DEFAULT FALSE,
			error_msg    TEXT
		);
	`)
	if err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

// UpsertContracts inserts or updates contracts by campfire_id.
func (db *DB) UpsertContracts(contracts []models.Contract) (int, error) {
	if len(contracts) == 0 {
		return 0, nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO contracts (
			campfire_id, client_name, deal_name, deal_id, status,
			currency, billing_frequency, contract_start_date, contract_end_date,
			closed_date, total_contract_value, total_billed, total_mrr,
			arr, arr_usd, exchange_rate, contract_months, is_evergreen,
			opportunity_id, last_modified_at, synced_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		)
		ON CONFLICT (campfire_id) DO UPDATE SET
			client_name          = EXCLUDED.client_name,
			deal_name            = EXCLUDED.deal_name,
			deal_id              = EXCLUDED.deal_id,
			status               = EXCLUDED.status,
			currency             = EXCLUDED.currency,
			billing_frequency    = EXCLUDED.billing_frequency,
			contract_start_date  = EXCLUDED.contract_start_date,
			contract_end_date    = EXCLUDED.contract_end_date,
			closed_date          = EXCLUDED.closed_date,
			total_contract_value = EXCLUDED.total_contract_value,
			total_billed         = EXCLUDED.total_billed,
			total_mrr            = EXCLUDED.total_mrr,
			arr                  = EXCLUDED.arr,
			arr_usd              = EXCLUDED.arr_usd,
			exchange_rate        = EXCLUDED.exchange_rate,
			contract_months      = EXCLUDED.contract_months,
			is_evergreen         = EXCLUDED.is_evergreen,
			opportunity_id       = EXCLUDED.opportunity_id,
			last_modified_at     = EXCLUDED.last_modified_at,
			synced_at            = EXCLUDED.synced_at
	`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, c := range contracts {
		var startDate, endDate, closedDate interface{}
		if c.ContractStartDate != "" {
			startDate = c.ContractStartDate
		}
		if c.ContractEndDate != "" {
			endDate = c.ContractEndDate
		}
		if c.ClosedDate != "" {
			closedDate = c.ClosedDate
		}

		_, err := stmt.Exec(
			c.CampfireID, c.ClientName, c.DealName, c.DealID, c.Status,
			c.Currency, c.BillingFrequency, startDate, endDate,
			closedDate, c.TotalContractValue, c.TotalBilled, c.TotalMRR,
			c.ARR, c.ARRUSD, c.ExchangeRate, c.ContractMonths, c.IsEvergreen,
			c.OpportunityID, c.LastModifiedAt, c.SyncedAt,
		)
		if err != nil {
			return count, fmt.Errorf("upserting contract %d: %w", c.CampfireID, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return count, nil
}

// ListContracts returns contracts optionally filtered by status.
func (db *DB) ListContracts(statusFilter string) ([]models.Contract, error) {
	query := `
		SELECT
			id, campfire_id, client_name, deal_name, deal_id, status,
			currency, billing_frequency,
			COALESCE(contract_start_date::text, ''),
			COALESCE(contract_end_date::text, ''),
			COALESCE(closed_date::text, ''),
			total_contract_value, total_billed, total_mrr,
			arr, arr_usd, exchange_rate, contract_months, is_evergreen,
			COALESCE(opportunity_id, ''), last_modified_at, synced_at
		FROM contracts
	`
	args := []interface{}{}
	if statusFilter != "" && statusFilter != "ALL" {
		query += " WHERE status = $1"
		args = append(args, statusFilter)
	}
	query += " ORDER BY arr_usd DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying contracts: %w", err)
	}
	defer rows.Close()

	var contracts []models.Contract
	for rows.Next() {
		var c models.Contract
		if err := rows.Scan(
			&c.ID, &c.CampfireID, &c.ClientName, &c.DealName, &c.DealID, &c.Status,
			&c.Currency, &c.BillingFrequency,
			&c.ContractStartDate, &c.ContractEndDate, &c.ClosedDate,
			&c.TotalContractValue, &c.TotalBilled, &c.TotalMRR,
			&c.ARR, &c.ARRUSD, &c.ExchangeRate, &c.ContractMonths, &c.IsEvergreen,
			&c.OpportunityID, &c.LastModifiedAt, &c.SyncedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning contract row: %w", err)
		}
		contracts = append(contracts, c)
	}
	return contracts, rows.Err()
}

// GetSummary returns aggregated ARR metrics.
func (db *DB) GetSummary() (models.Summary, error) {
	var s models.Summary

	err := db.conn.QueryRow(`
		SELECT
			COALESCE(SUM(arr_usd), 0)                              AS total_arr_usd,
			COALESCE(SUM(arr_usd) / 12.0, 0)                      AS total_mrr_usd,
			COUNT(*)                                                AS active_contracts,
			COUNT(*) FILTER (WHERE is_evergreen)                   AS evergreen_contracts
		FROM contracts
		WHERE status = 'ACTIVE'
		  AND contract_start_date <= CURRENT_DATE
		  AND (contract_end_date >= CURRENT_DATE OR is_evergreen = true)
	`).Scan(&s.TotalARRUSD, &s.TotalMRRUSD, &s.ActiveContracts, &s.EvergreenContracts)
	if err != nil {
		return s, fmt.Errorf("querying summary: %w", err)
	}
	s.ContractCount = s.ActiveContracts

	rows, err := db.conn.Query(`
		SELECT currency, COALESCE(SUM(arr),0), COALESCE(SUM(arr_usd),0), COUNT(*)
		FROM contracts
		WHERE status = 'ACTIVE'
		  AND contract_start_date <= CURRENT_DATE
		  AND (contract_end_date >= CURRENT_DATE OR is_evergreen = true)
		GROUP BY currency
		ORDER BY SUM(arr_usd) DESC
	`)
	if err != nil {
		return s, fmt.Errorf("querying currency breakdown: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ca models.CurrencyARR
		if err := rows.Scan(&ca.Currency, &ca.ARR, &ca.ARRUSD, &ca.Count); err != nil {
			return s, err
		}
		s.ByCurrency = append(s.ByCurrency, ca)
	}

	// Last sync time
	var lastSync sql.NullTime
	_ = db.conn.QueryRow(`SELECT MAX(synced_at) FROM sync_log WHERE error_msg IS NULL`).Scan(&lastSync)
	if lastSync.Valid {
		s.LastSyncedAt = &lastSync.Time
	}

	return s, nil
}

// LogSync records a sync operation result.
func (db *DB) LogSync(result models.SyncResult, errMsg string) error {
	var errPtr interface{}
	if errMsg != "" {
		errPtr = errMsg
	}
	_, err := db.conn.Exec(
		`INSERT INTO sync_log (synced_at, upserted, total, incremental, error_msg)
		 VALUES ($1, $2, $3, $4, $5)`,
		result.SyncedAt, result.Upserted, result.Total, result.Incremental, errPtr,
	)
	return err
}

// LastSyncTime returns the most recent successful sync time for incremental refresh.
func (db *DB) LastSyncTime() (*time.Time, error) {
	var t sql.NullTime
	err := db.conn.QueryRow(
		`SELECT MAX(synced_at) FROM sync_log WHERE error_msg IS NULL`,
	).Scan(&t)
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}
