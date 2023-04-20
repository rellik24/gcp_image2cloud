package cloudsql

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	indexTmpl = template.Must(template.New("index").Parse(indexHTML))
	db        *sql.DB
	once      sync.Once
	uid       int
	username  string
)

// getDB lazily instantiates a database connection pool. Users of Cloud Run or
// Cloud Functions may wish to skip this lazy instantiation and connect as soon
// as the function is loaded. This is primarily to help testing.
func getDB() *sql.DB {
	once.Do(func() {
		db = mustConnect()
	})
	return db
}

// migrateDB creates the votes table if it does not already exist.
func migrateDB(db *sql.DB) error {
	if _, err := db.Exec("DROP TABLE IF EXISTS Image;"); err != nil {
		log.Fatalf("DB.Exec: unable to drop Image table: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS Members;"); err != nil {
		log.Fatalf("DB.Exec: unable to drop User table: %v", err)
	}

	// Create the user table.
	createUser := `CREATE TABLE Members (
		UID int IDENTITY(1,1) PRIMARY KEY,
		Account nvarchar(50) UNIQUE NOT NULL,
		Username nvarchar(50) NOT NULL,
		PWD nvarchar(255) NOT NULL,
		Created_at DATETIME NOT NULL
	);`
	if _, err := db.Exec(createUser); err != nil {
		return err
	}

	// Create the images table.
	createImage := `CREATE TABLE Image (
		ID int IDENTITY(1,1) PRIMARY KEY,
		IName nvarchar(50) NOT NULL,
		FileSize nvarchar(50) NOT NULL,
		Created_at DATETIME NOT NULL,
		Link nvarchar(255) NOT NULL,
		UID int REFERENCES Members (UID)
	);`
	_, err := db.Exec(createImage)
	return err
}

// recentVotes returns the last five votes cast.
func recentVotes(db *sql.DB) ([]vote, error) {
	rows, err := db.Query("SELECT TOP 5 RTRIM(candidate), created_at FROM votes ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("DB.Query: %v", err)
	}
	defer rows.Close()

	var votes []vote
	for rows.Next() {
		var (
			candidate string
			voteTime  time.Time
		)
		err := rows.Scan(&candidate, &voteTime)
		if err != nil {
			return nil, fmt.Errorf("Rows.Scan: %v", err)
		}
		votes = append(votes, vote{Candidate: candidate, VoteTime: voteTime})
	}
	return votes, nil
}

// formatMargin calculates the difference between votes and returns a human
// friendly margin (e.g., 2 votes)
func formatMargin(a, b int) string {
	diff := int(math.Abs(float64(a - b)))
	margin := fmt.Sprintf("%d votes", diff)
	// remove pluralization when diff is just one
	if diff == 1 {
		margin = "1 vote"
	}
	return margin
}

// votingData is used to pass data to the HTML template.
type votingData struct {
	TabsCount   int
	SpacesCount int
	VoteMargin  string
	RecentVotes []vote
}

// currentTotals retrieves all voting data from the database.
func currentTotals(db *sql.DB) (votingData, error) {
	var (
		tabs   int
		spaces int
	)
	err := db.QueryRow("SELECT count(id) FROM votes WHERE candidate='TABS'").Scan(&tabs)
	if err != nil {
		return votingData{}, fmt.Errorf("DB.QueryRow: %v", err)
	}
	err = db.QueryRow("SELECT count(id) FROM votes WHERE candidate='SPACES'").Scan(&spaces)
	if err != nil {
		return votingData{}, fmt.Errorf("DB.QueryRow: %v", err)
	}

	recent, err := recentVotes(db)
	if err != nil {
		return votingData{}, fmt.Errorf("recentVotes: %v", err)
	}

	return votingData{
		TabsCount:   tabs,
		SpacesCount: spaces,
		VoteMargin:  formatMargin(tabs, spaces),
		RecentVotes: recent,
	}, nil
}

// mustConnect creates a connection to the database based on environment
// variables. Setting one of INSTANCE_HOST or INSTANCE_CONNECTION_NAME will
// establish a connection using a TCP socket or a connector respectively.
func mustConnect() *sql.DB {
	var (
		db  *sql.DB
		err error
	)

	// Use a TCP socket when INSTANCE_HOST (e.g., 127.0.0.1) is defined
	if os.Getenv("INSTANCE_HOST") != "" {
		db, err = connectTCPSocket()
		if err != nil {
			log.Fatalf("connectTCPSocket: unable to connect: %s", err)
		}
	}

	// Use the connector when INSTANCE_CONNECTION_NAME (proj:region:instance) is defined.
	if os.Getenv("INSTANCE_CONNECTION_NAME") != "" {
		db, err = connectWithConnector()
		if err != nil {
			log.Fatalf("connectConnector: unable to connect: %s", err)
		}
	}

	if db == nil {
		log.Fatal("Missing database connection type. Please define one of INSTANCE_HOST or INSTANCE_CONNECTION_NAME")
	}

	// if err := migrateDB(db); err != nil {
	// 	log.Fatalf("unable to create table: %s", err)
	// }

	return db
}

// configureConnectionPool sets database connection pool properties.
// For more information, see https://golang.org/pkg/database/sql
func configureConnectionPool(db *sql.DB) {
	// [START cloud_sql_sqlserver_databasesql_limit]
	// Set maximum number of connections in idle connection pool.
	db.SetMaxIdleConns(5)

	// Set maximum number of open connections to the database.
	db.SetMaxOpenConns(7)
	// [END cloud_sql_sqlserver_databasesql_limit]

	// [START cloud_sql_sqlserver_databasesql_lifetime]
	// Set Maximum time (in seconds) that a connection can remain open.
	db.SetConnMaxLifetime(1800 * time.Second)
	// [END cloud_sql_sqlserver_databasesql_lifetime]

	// [START cloud_sql_sqlserver_databasesql_backoff]
	// database/sql does not support specifying backoff
	// [END cloud_sql_sqlserver_databasesql_backoff]
	// [START cloud_sql_sqlserver_databasesql_timeout]
	// The database/sql package currently doesn't offer any functionality to
	// configure connection timeout.
	// [END cloud_sql_sqlserver_databasesql_timeout]
}

// Votes handles HTTP requests to alternatively show the voting app or to save a
// vote.
func Votes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		renderIndex(w, r, getDB())
	case http.MethodPost:
		PostProcess(w, r, getDB())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// vote contains a single row from the votes table in the database. Each vote
// includes a candidate ("TABS" or "SPACES") and a timestamp.
type vote struct {
	Candidate string
	VoteTime  time.Time
}

// renderIndex renders the HTML application with the voting form, current
// totals, and recent votes.
func renderIndex(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	t, err := currentTotals(db)
	if err != nil {
		log.Printf("renderIndex: failed to read current totals: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = indexTmpl.Execute(w, t)
	if err != nil {
		log.Printf("renderIndex: failed to render template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// PostProcess:
func PostProcess(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if err := r.ParseForm(); err != nil {
		log.Printf("AddUser: failed to parse form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	queryName := strings.Split(r.RequestURI, "?")[0]
	switch queryName {
	case "/addUser":
		usr := r.FormValue("username")
		account := r.FormValue("account")
		pwd := r.FormValue("pwd")
		if usr == "" || account == "" || pwd == "" {
			log.Printf("Add member error")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// [START cloud_sql_sqlserver_databasesql_connection]
		addUser := "INSERT INTO Members (account, username, pwd, created_at) VALUES (@account, @username, @pwd, GETDATE())"
		_, err := db.Exec(addUser, sql.Named("account", account), sql.Named("username", usr), sql.Named("pwd", pwd))
		// [END cloud_sql_sqlserver_databasesql_connection]

		if err != nil {
			log.Printf("Error: unable to add user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Member successfully add: %s!", usr)

	case "/verifyUser":
		account := r.FormValue("account")
		pwd := r.FormValue("pwd")
		if account == "" || pwd == "" {
			log.Printf("Account or Password should not be empty.")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		verifyUser := "EXEC dbo.VerifyUser @account, @pwd"
		if err := db.QueryRow(verifyUser, sql.Named("account", account), sql.Named("pwd", pwd)).Scan(&uid, &username); err != nil {
			log.Printf("Error: unable to add user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Member successfully verify: Hi %s !", username)

	default:
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

var indexHTML = `
<html lang="en">
<head>
    <title>Tabs VS Spaces</title>
    <link rel="icon" type="image/png" href="data:image/png;base64,iVBORw0KGgo=">
    <link rel="stylesheet"
          href="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css">
    <link href="https://fonts.googleapis.com/icon?family=Material+Icons" rel="stylesheet">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/js/materialize.min.js"></script>
</head>
<body>
<nav class="red lighten-1">
    <div class="nav-wrapper">
        <a href="#" class="brand-logo center">Tabs VS Spaces</a>
    </div>
</nav>
<div class="section">
    <div class="center">
        <h4>
            {{ if eq .TabsCount .SpacesCount }}
                TABS and SPACES are evenly matched!
            {{ else if gt .TabsCount .SpacesCount }}
                TABS are winning by {{ .VoteMargin }}
            {{ else if gt .SpacesCount .TabsCount }}
                SPACES are winning by {{ .VoteMargin }}
            {{ end }}
        </h4>
    </div>
    <div class="row center">
        <div class="col s6 m5 offset-m1">
            {{ if gt .TabsCount .SpacesCount }}
			<div class="card-panel green lighten-3">
			{{ else }}
			<div class="card-panel">
			{{ end }}
                <i class="material-icons large">keyboard_tab</i>
                <h3>{{ .TabsCount }} votes</h3>
                <button id="voteTabs" class="btn green">Vote for TABS</button>
            </div>
        </div>
        <div class="col s6 m5">
            {{ if lt .TabsCount .SpacesCount }}
			<div class="card-panel blue lighten-3">
			{{ else }}
			<div class="card-panel">
			{{ end }}
                <i class="material-icons large">space_bar</i>
                <h3>{{ .SpacesCount }} votes</h3>
                <button id="voteSpaces" class="btn blue">Vote for SPACES</button>
            </div>
        </div>
    </div>
    <h4 class="header center">Recent Votes</h4>
    <ul class="container collection center">
        {{ range .RecentVotes }}
            <li class="collection-item avatar">
                {{ if eq .Candidate "TABS" }}
                    <i class="material-icons circle green">keyboard_tab</i>
                {{ else if eq .Candidate "SPACES" }}
                    <i class="material-icons circle blue">space_bar</i>
                {{ end }}
                <span class="title">
                    A vote for <b>{{.Candidate}}</b> was cast at {{.VoteTime.Format "2006-01-02T15:04:05Z07:00" }}
                </span>
            </li>
        {{ end }}
    </ul>
</div>
<script>
    function vote(team) {
        var xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function () {
            if (this.readyState == 4) {
                window.location.reload();
				console.log(344)
            }
        };
        xhr.open("POST", "/Votes", true);
        xhr.setRequestHeader("Content-Type", "application/x-www-form-urlencoded");
        xhr.send("team=" + team);
    }

    document.getElementById("voteTabs").addEventListener("click", function () {
        vote("TABS");
    });
    document.getElementById("voteSpaces").addEventListener("click", function () {
        vote("SPACES");
    });
</script>
</body>
</html>
`
