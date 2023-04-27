package cloudsql

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rellik24/image2cloud/cloudkey"
	"github.com/rellik24/image2cloud/cloudstorage"
)

var (
	indexTmpl = template.Must(template.New("index").Parse(indexHTML))
	db        *sql.DB
	once      sync.Once
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
		HashName nvarchar(65) NOT NULL,
		Link nvarchar(255) NOT NULL,
		FileSize nvarchar(50) NOT NULL,
		Created_at DATETIME NOT NULL,
		UID int REFERENCES Members (UID)
	);`
	_, err := db.Exec(createImage)
	return err
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

// API function handles HTTP requests
func API(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getProcess(w, r, getDB())
	case http.MethodPost:
		postProcess(w, r, getDB())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// getProcess:
func getProcess(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if err := r.ParseForm(); err != nil {
		log.Printf("AddUser: failed to parse form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	queryName := strings.Split(r.RequestURI, "?")[0]
	switch queryName {
	case "/api/download":
		token := r.FormValue("token")
		_, err := authToken(token)
		if err != nil {
			return
		}
		filename := r.FormValue("filename")
		if err := cloudstorage.DownloadFile(w, filename, "download"); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println(err.Error())
			return
		}
	}
}

// postProcess:
func postProcess(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if err := r.ParseForm(); err != nil {
		log.Printf("AddUser: failed to parse form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	queryName := strings.Split(r.RequestURI, "?")[0]
	switch queryName {
	case "/api/addUser":
		usr := r.FormValue("username")
		account := r.FormValue("account")
		pwd := r.FormValue("pwd")
		if usr == "" || account == "" || pwd == "" {
			log.Printf("Add member error")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pwd, err := cloudkey.SignMac(w, pwd)
		if err != nil {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		// [START cloud_sql_sqlserver_databasesql_connection]
		addUser := "INSERT INTO Members (account, username, pwd, created_at) VALUES (@account, @username, @pwd, GETDATE())"
		_, err = db.Exec(addUser, sql.Named("account", account), sql.Named("username", usr), sql.Named("pwd", pwd))
		// [END cloud_sql_sqlserver_databasesql_connection]

		if err != nil {
			log.Printf("Error: unable to add user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Member successfully add: %s!", usr)

	case "/api/login":
		account := r.FormValue("account")
		pwd := r.FormValue("pwd")
		if account == "" || pwd == "" {
			log.Printf("Account or Password should not be empty.")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pwd, err := cloudkey.SignMac(w, pwd)
		if err != nil {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		verifyUser := "EXEC dbo.VerifyUser @account, @pwd"

		var uid int
		var username string

		if err := db.QueryRow(verifyUser, sql.Named("account", account), sql.Named("pwd", pwd)).Scan(&uid, &username); err != nil {
			log.Printf("Error: unable to add user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "Member successfully verify: Hi %s !", username)

		str, err := cloudkey.CreateToken(uid, account, username)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Printf("JSON Web Token %s \n!", str)

	case "/api/getImage", "/api/upload", "/api/download":
		token := r.FormValue("token")
		account, err := authToken(token)
		if err != nil {
			return
		}
		switch queryName {
		case "/api/download":
			filename := r.FormValue("filename")
			if err := cloudstorage.DownloadFile(w, filename, "output.png"); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}

		case "/api/upload":
			filename := r.FormValue("filename")
			if err := cloudstorage.UploadFile(w, account, filename); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}

			uploadFile := "exec dbo.InsertImage @account, @filename"
			if _, err := db.Exec(uploadFile, sql.Named("account", account), sql.Named("filename", filename)); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}
		}

	default:
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// authToken :
func authToken(token string) (string, error) {
	claim, err := cloudkey.ValidateToken(token)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	account, ok := claim["account"].(string)
	if !ok {
		return "", errors.New("parse token error")
	}
	return account, nil
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
