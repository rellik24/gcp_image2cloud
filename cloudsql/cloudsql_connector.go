// [START cloud_sql_sqlserver_databasesql_connect_connector]
package cloudsql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"cloud.google.com/go/cloudsqlconn"
	mssql "github.com/denisenkom/go-mssqldb"
)

type csqlDialer struct {
	dialer     *cloudsqlconn.Dialer
	connName   string
	usePrivate bool
}

// DialContext adheres to the mssql.Dialer interface.
func (c *csqlDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var opts []cloudsqlconn.DialOption
	if c.usePrivate {
		opts = append(opts, cloudsqlconn.WithPrivateIP())
	}
	return c.dialer.Dial(ctx, c.connName, opts...)
}

func connectWithConnector() (*sql.DB, error) {
	mustGetenv := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			log.Fatalf("Fatal Error in connect_connector.go: %s environment variable not set.\n", k)
		}
		return v
	}
	// Note: Saving credentials in environment variables is convenient, but not
	// secure - consider a more secure solution such as
	// Cloud Secret Manager (https://cloud.google.com/secret-manager) to help
	// keep secrets safe.
	var (
		dbUser                 = mustGetenv("DB_USER")                  // e.g. 'my-db-user'
		dbPwd                  = mustGetenv("DB_PASS")                  // e.g. 'my-db-password'
		dbName                 = mustGetenv("DB_NAME")                  // e.g. 'my-database'
		instanceConnectionName = mustGetenv("INSTANCE_CONNECTION_NAME") // e.g. 'project:region:instance'
		usePrivate             = os.Getenv("PRIVATE_IP")
	)

	dbURI := fmt.Sprintf("user id=%s;password=%s;database=%s;", dbUser, dbPwd, dbName)
	c, err := mssql.NewConnector(dbURI)
	if err != nil {
		return nil, fmt.Errorf("mssql.NewConnector: %v", err)
	}
	dialer, err := cloudsqlconn.NewDialer(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cloudsqlconn.NewDailer: %v", err)
	}
	c.Dialer = &csqlDialer{
		dialer:     dialer,
		connName:   instanceConnectionName,
		usePrivate: usePrivate != "",
	}

	dbPool := sql.OpenDB(c)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %v", err)
	}
	return dbPool, nil
}

// [END cloud_sql_sqlserver_databasesql_connect_connector]
