// Package d1http exposes Cloudflare D1's HTTPS API through database/sql.
//
// It deliberately supports ordinary queries and writes only. D1 does not offer
// an interactive transaction that can safely implement database/sql's *sql.Tx
// contract, so Begin always returns ErrTransactionsUnsupported.
package d1http

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrTransactionsUnsupported = errors.New("d1http: interactive transactions are unsupported; use a D1 batch or a guarded update")

type Config struct {
	AccountID  string
	DatabaseID string
	APIToken   string
	HTTPClient *http.Client
	Endpoint   string
}

func Open(cfg Config) (*sql.DB, error) {
	connector, err := NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func NewConnector(cfg Config) (*Connector, error) {
	if cfg.AccountID == "" || cfg.DatabaseID == "" || cfg.APIToken == "" {
		return nil, errors.New("d1http: account ID, database ID, and API token are required")
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(cfg.AccountID) + "/d1/database/" + url.PathEscape(cfg.DatabaseID) + "/raw"
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("d1http: invalid endpoint: %q", endpoint)
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Connector{endpoint: endpoint, token: cfg.APIToken, client: client}, nil
}

type Connector struct {
	endpoint string
	token    string
	client   *http.Client
}

func (c *Connector) Connect(context.Context) (driver.Conn, error) {
	return &conn{connector: c}, nil
}

func (c *Connector) Driver() driver.Driver { return drv{} }

type drv struct{}

func (drv) Open(string) (driver.Conn, error) {
	return nil, errors.New("d1http: use d1http.Open with Config")
}

type conn struct{ connector *Connector }

func (c *conn) Prepare(query string) (driver.Stmt, error) { return &stmt{conn: c, query: query}, nil }
func (c *conn) Close() error                              { return nil }
func (c *conn) Begin() (driver.Tx, error)                 { return nil, ErrTransactionsUnsupported }
func (c *conn) Ping(ctx context.Context) error {
	_, err := c.query(ctx, "SELECT 1", nil)
	return err
}

func (c *conn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return nil, ErrTransactionsUnsupported
}

func (c *conn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return c.Prepare(query)
}

func (c *conn) CheckNamedValue(value *driver.NamedValue) error {
	if value.Value == nil {
		return nil
	}
	if t, ok := value.Value.(time.Time); ok {
		value.Value = t.UTC().Format(time.RFC3339Nano)
		return nil
	}
	converted, err := driver.DefaultParameterConverter.ConvertValue(value.Value)
	if err != nil {
		return err
	}
	value.Value = converted
	return nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return d1Result{changes: result.Meta.Changes, lastInsertID: result.lastRowID()}, nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	result, err := c.query(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return &rows{columns: result.Results.Columns, values: result.Results.Rows}, nil
}

func (c *conn) query(ctx context.Context, query string, args []driver.NamedValue) (queryResult, error) {
	params := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Ordinal != i+1 {
			return queryResult{}, errors.New("d1http: named parameters are unsupported")
		}
		params[i] = arg.Value
	}
	body, err := json.Marshal(queryRequest{SQL: query, Params: params})
	if err != nil {
		return queryResult{}, fmt.Errorf("d1http: encode query: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.connector.endpoint, bytes.NewReader(body))
	if err != nil {
		return queryResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.connector.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.connector.client.Do(req)
	if err != nil {
		return queryResult{}, fmt.Errorf("d1http: request: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return queryResult{}, fmt.Errorf("d1http: read response: %w", err)
	}
	var payload apiResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return queryResult{}, fmt.Errorf("d1http: decode response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !payload.Success || len(payload.Result) != 1 || !payload.Result[0].Success {
		return queryResult{}, apiError{status: response.StatusCode, messages: payload.errorMessages()}
	}
	return payload.Result[0], nil
}

type stmt struct {
	conn  *conn
	query string
}

func (s *stmt) Close() error  { return nil }
func (s *stmt) NumInput() int { return -1 }
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.conn.ExecContext(context.Background(), s.query, toNamed(args))
}
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.conn.QueryContext(context.Background(), s.query, toNamed(args))
}
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.conn.ExecContext(ctx, s.query, args)
}
func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.conn.QueryContext(ctx, s.query, args)
}

func toNamed(args []driver.Value) []driver.NamedValue {
	result := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		result[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return result
}

type rows struct {
	columns []string
	values  [][]any
	index   int
}

func (r *rows) Columns() []string { return r.columns }
func (r *rows) Close() error      { return nil }
func (r *rows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	for i, value := range r.values[r.index] {
		converted, err := toDriverValue(value)
		if err != nil {
			return err
		}
		dest[i] = converted
	}
	r.index++
	return nil
}

func toDriverValue(value any) (driver.Value, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string, bool, float64:
		return typed, nil
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer, nil
		}
		return typed.Float64()
	default:
		return nil, fmt.Errorf("d1http: unsupported D1 value %T", value)
	}
}

type d1Result struct {
	changes      int64
	lastInsertID int64
}

func (r d1Result) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r d1Result) RowsAffected() (int64, error) { return r.changes, nil }

type queryRequest struct {
	SQL    string         `json:"sql"`
	Params []driver.Value `json:"params,omitempty"`
}

type apiResponse struct {
	Success bool          `json:"success"`
	Errors  []apiIssue    `json:"errors"`
	Result  []queryResult `json:"result"`
}

func (r apiResponse) errorMessages() string {
	parts := make([]string, 0, len(r.Errors))
	for _, issue := range r.Errors {
		parts = append(parts, issue.Message)
	}
	if len(parts) == 0 {
		return "D1 rejected query"
	}
	return strings.Join(parts, "; ")
}

type apiIssue struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type queryResult struct {
	Success bool `json:"success"`
	Meta    struct {
		Changes   int64       `json:"changes"`
		LastRowID json.Number `json:"last_row_id"`
	} `json:"meta"`
	Results struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	} `json:"results"`
}

func (r queryResult) lastRowID() int64 {
	if r.Meta.LastRowID == "" {
		return 0
	}
	id, _ := strconv.ParseInt(string(r.Meta.LastRowID), 10, 64)
	return id
}

type apiError struct {
	status   int
	messages string
}

func (e apiError) Error() string { return fmt.Sprintf("d1http: HTTP %d: %s", e.status, e.messages) }

var _ driver.Connector = (*Connector)(nil)
var _ driver.Conn = (*conn)(nil)
var _ driver.ConnBeginTx = (*conn)(nil)
var _ driver.ConnPrepareContext = (*conn)(nil)
var _ driver.ExecerContext = (*conn)(nil)
var _ driver.NamedValueChecker = (*conn)(nil)
var _ driver.Pinger = (*conn)(nil)
var _ driver.QueryerContext = (*conn)(nil)
