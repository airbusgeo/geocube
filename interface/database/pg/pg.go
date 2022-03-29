package pg

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/internal/geocube"

	"github.com/lib/pq"
)

// pgInterface allows to use either a sql.DB or a sql.Tx
type pgInterface interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// BackendTx implements GeocubeTxBackend
type BackendTx struct {
	*sql.Tx
	Backend
}

// BackendDB implements GeocubeDBBackend
type BackendDB struct {
	*sql.DB
	Backend
}

// Backend implements GeocubeBackend
type Backend struct {
	pg pgInterface
}

// StartTransaction implements GeocubeDBBackend
func (bdb BackendDB) StartTransaction(ctx context.Context) (database.GeocubeTxBackend, error) {
	tx, err := bdb.BeginTx(ctx, nil)
	if err != nil {
		return BackendTx{}, err
	}
	return BackendTx{tx, Backend{pg: tx}}, nil
}

// Rollback overloads sql.Tx.Rollback to be idempotent
func (btx BackendTx) Rollback() error {
	err := btx.Tx.Rollback()
	if err == sql.ErrTxDone {
		return nil
	}
	return err
}

type Credentials struct {
	Cert   string `json:"apiserver.crt"`
	Key    string `json:"apiserver.key"`
	CaCert string `json:"root.crt"`
}

func createPemFile(filename, contents string) error {
	if len(contents) == 0 {
		return fmt.Errorf("no contents")
	}
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create %s: %w", filename, err)
	}
	err = os.Chmod(filename, 0600)
	if err != nil {
		return fmt.Errorf("chmod %s: %w", filename, err)
	}
	f.WriteString(contents)
	err = f.Close()
	if err != nil {
		return fmt.Errorf("close %s: %w", filename, err)
	}
	return nil
}

func ConnStringFromId(dbName, dbUser, dbHost, dbPassword string) (string, error) {
	if dbName == "" {
		return "", fmt.Errorf("missing dbName flag")
	}

	if dbUser == "" {
		return "", fmt.Errorf("missing dbUser flag")
	}

	if dbHost == "" {
		return "", fmt.Errorf("missing dbHost flag")
	}

	if dbPassword == "" {
		return "", fmt.Errorf("missing dbPassword flag")
	}

	return fmt.Sprintf("postgres://%s:%s@%s/%s?binary_parameters=yes", dbUser, dbPassword, dbHost, dbName), nil
}

func ConnStringFromCertFiles(dbName, dbUser, dbHost, certFile, keyFile, caCertFile string) (string, error) {
	if dbName == "" {
		return "", fmt.Errorf("missing dbName flag")
	}

	if dbUser == "" {
		return "", fmt.Errorf("missing dbUser flag")
	}

	if dbHost == "" {
		return "", fmt.Errorf("missing dbHost flag")
	}

	return fmt.Sprintf("postgres://%s@%s/%s?sslmode=require&sslcert=%s&sslkey=%s&sslrootcert=%s&binary_parameters=yes",
		dbUser, dbHost, dbName, certFile, keyFile, caCertFile), nil
}

func ConnStringFromCredentials(dbName, dbUser, dbHost string, creds Credentials) (string, error) {
	tempDir, err := ioutil.TempDir("/tmp", "")
	if err != nil {
		return "", fmt.Errorf("tempdir: %w", err)
	}
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")
	caCertFile := filepath.Join(tempDir, "cacert.pem")
	if err = createPemFile(certFile, creds.Cert); err != nil {
		return "", fmt.Errorf("create cert: %w", err)
	}
	if err = createPemFile(keyFile, creds.Key); err != nil {
		return "", fmt.Errorf("create key: %w", err)
	}
	if err = createPemFile(caCertFile, creds.CaCert); err != nil {
		return "", fmt.Errorf("create cacert: %w", err)
	}
	return ConnStringFromCertFiles(dbUser, dbHost, dbName, certFile, keyFile, caCertFile)
}

func New(ctx context.Context, dbConnection string) (*BackendDB, error) {
	db, err := sql.Open("postgres", dbConnection)
	if err != nil {
		return nil, fmt.Errorf("sql.open: %w", err)
	}
	db.SetMaxOpenConns(5)
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &BackendDB{db, Backend{pg: db}}, nil
}

func scanIdsAndClose(rows *sql.Rows) (IDs []string, err error) {
	defer func() {
		if e := rows.Close(); e != nil && err == nil {
			err = e
		}
	}()

	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("db.scanIdsAndClose: %w", err)
		}
		IDs = append(IDs, id)
	}
	return IDs, nil
}

func (b Backend) bulkInsert(ctx context.Context, schema, table string, columns []string, data [][]interface{}) (err error) {
	// Prepare the insert
	stmt, err := b.pg.PrepareContext(ctx, pq.CopyInSchema(schema, table, columns...))
	if err != nil {
		return pqErrorFormat("db.createtasks.prepare: %w", err)
	}
	defer func() {
		if e := stmt.Close(); e != nil && err == nil {
			err = e
		}
	}()

	// Append the datas
	for _, d := range data {
		if _, err := stmt.ExecContext(ctx, d...); err != nil {
			return err
		}
	}

	// Execute statement
	_, err = stmt.ExecContext(ctx)
	return err
}

func (b Backend) delete(ctx context.Context, table, key, id string) error {
	_, err := b.pg.ExecContext(ctx, "DELETE FROM geocube."+table+" WHERE "+key+" = $1", id)
	switch pqErrorCode(err) {
	case noError:
		return nil
	case foreignKeyViolation:
		table2 := ""
		if key, id = extractKeyValueFromDetail(err.(*pq.Error)); id != "" {
			if tableVal := regexp.MustCompile("table \"(.*)\".").FindStringSubmatch(err.(*pq.Error).Detail); len(tableVal) == 2 {
				table2 = tableVal[1]
			}
		}
		return geocube.NewDependencyStillExists(table, table2, key, id, "")
	default:
		return pqErrorFormat(fmt.Sprintf("db.delete%s: %%w", table), err)
	}
}

func limitOffsetClause(page, limit int) string {
	if limit != 0 {
		if page != 0 {
			return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, page*limit)
		}
		return fmt.Sprintf(" LIMIT %d", limit)
	}
	return ""
}

// preserveOrder returns a map giving the index of the id in the list (removing duplicated id)
func preserveOrder(ids []string) map[string]int {
	idx := map[string]int{}
	for _, id := range ids {
		if _, ok := idx[id]; !ok {
			idx[id] = len(idx)
		}
	}
	return idx
}

// parseString to be used by LIKE
// * will be replace by %, "?" by "_", "_" by "\\_" and (?i) suffix for case-insensitivity
// Return false if the string does not have ? or *
func parseString(s string) (string, bool) {
	s = strings.ReplaceAll(s, "_", "\\_")
	news := strings.ReplaceAll(strings.ReplaceAll(s, "*", "%"), "?", "_")
	return news, s != news
}

// parse value to be used by LIKE
// * will be replace by %, "?" by "_" and (?i) suffix for case-insensitivity
// Return operator =, LIKE or ILIKE
func parseLike(value string) (string, string) {
	if strings.HasSuffix(value, "(?i)") {
		s, _ := parseString(value[0 : len(value)-4])
		return s, "ILIKE"
	}
	if newv, parsed := parseString(value); parsed {
		return newv, "LIKE"
	}
	return value, "="
}

// parse value to be used by LIKE
// * will be replace by %, "?" by "_" and (?i) suffix for case-insensitivity
// Return values to be used with equal, LIKE or ILIKE
func parseLikes(values []string) ([]string, []string, []string) {
	var equals, likes, ilikes []string
	for _, value := range values {
		if strings.HasSuffix(value, "(?i)") {
			value, _ = parseString(value[0 : len(value)-4])
			ilikes = append(ilikes, value)
		} else if newvalue, parsed := parseString(value); parsed {
			likes = append(likes, newvalue)
		} else {
			equals = append(equals, value)
		}
	}
	return equals, likes, ilikes
}

type joinClause struct {
	Parameters []interface{}
	clause     []string
}

func (wc *joinClause) append(clause string, parameters ...interface{}) {
	positions := []interface{}{}
	for i := range parameters {
		positions = append(positions, len(wc.Parameters)+i+1)
	}

	wc.Parameters = append(wc.Parameters, parameters...)
	wc.clause = append(wc.clause, fmt.Sprintf(clause, positions...))
}

func (wc *joinClause) appendWithoutPlacement(clause string, parameters ...interface{}) {
	wc.Parameters = append(wc.Parameters, parameters...)
	wc.clause = append(wc.clause, clause)
}

func (wc joinClause) WhereClause() string {
	return wc.Clause(" WHERE ", " AND ", "")
}

func (wc joinClause) Clause(prefix, sep, suffix string) string {
	if len(wc.clause) > 0 {
		return prefix + strings.Join(wc.clause, sep) + suffix
	}
	return ""
}
