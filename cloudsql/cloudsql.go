package cloudsql

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rellik24/image2cloud/cloudimage"
	"github.com/rellik24/image2cloud/cloudkey"
	"github.com/rellik24/image2cloud/cloudstorage"
)

var (
	db   *sql.DB
	once sync.Once
)

type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	Result bool   `json:"result"`
}

// getDB lazily instantiates a database connection pool. Users of Cloud Run or
// Cloud Functions may wish to skip this lazy instantiation and connect as soon
// as the function is loaded. This is primarily to help testing.
func getDB() *sql.DB {
	once.Do(func() {
		db = mustConnect()
	})
	return db
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
		getHandler(w, r, getDB())
	case http.MethodPost:
		postHandler(w, r, getDB())
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type Image struct {
	Name    string `json:"name"`
	Created string `json:"created"`
	Link    string `json:"link"`
}

// getHandler:
func getHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if err := r.ParseForm(); err != nil {
		log.Printf("GET: failed to parse form: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	queryName := strings.Split(r.RequestURI, "?")[0]
	token := r.FormValue("token")
	account, err := authToken(token)
	if err != nil {
		return
	}
	switch queryName {
	case "/api/list":
		listQuery := "select i.IName, i.Created_at, i.Link from image i, Members m where m.Account = @account and m.uid = i.UID"
		rows, err := db.Query(listQuery, sql.Named("account", account))
		if err != nil {
			log.Printf("Error: unable to add user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		var result []Image
		for rows.Next() {
			var img Image
			err := rows.Scan(&img.Name, &img.Created, &img.Link)
			if err != nil {
				return
			}
			if t, err := time.Parse(time.RFC3339, img.Created); err == nil {
				img.Created = t.Format("2006-01-02 15:04:05")
			}
			result = append(result, img)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	case "/api/download":
		filename := r.FormValue("filename")
		downloadFile := "exec dbo.DownloadImage @account, @filename"
		var result int
		if err := db.QueryRow(downloadFile, sql.Named("account", account), sql.Named("filename", filename)).Scan(&result); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println(err.Error())
			return
		}
		if result != 0 {
			if err := cloudstorage.DownloadFile(w, fmt.Sprintf("%s/%s", account, filename), "download"); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("Invalid image name")
		}
	default:
		http.Error(w, "Invalid API", http.StatusBadRequest)
	}
}

// postHandler:
func postHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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
		var req LoginRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Account == "" || req.Password == "" {
			log.Printf("Account or Password should not be empty.")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pwd, err := cloudkey.SignMac(w, req.Password)
		if err != nil {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		verifyUser := "EXEC dbo.VerifyUser @account, @pwd"

		var uid int
		var username string
		resp := LoginResponse{Token: "", Result: false}
		w.Header().Set("Content-Type", "application/json")

		if err := db.QueryRow(verifyUser, sql.Named("account", req.Account), sql.Named("pwd", pwd)).Scan(&uid, &username); err != nil {
			log.Printf("Error: unable to login: %v", err)
			json.NewEncoder(w).Encode(resp)
			return
		}
		token, err := cloudkey.CreateToken(uid, req.Account, username)
		if err != nil {
			fmt.Println(err.Error())
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp = LoginResponse{Token: token, Result: true}
		json.NewEncoder(w).Encode(resp)
	case "/api/upload":
		token := r.FormValue("token")
		account, err := authToken(token)
		if err != nil {
			return
		}
		switch queryName {
		case "/api/upload":
			filename := r.FormValue("filename")
			// 壓縮檔案
			if err := cloudimage.Compress(filename); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}
			// 上傳
			if err := cloudstorage.UploadFile(w, account, filename); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}
			// 上傳成功記錄 DB
			uploadFile := "exec dbo.InsertImage @account, @filename"
			if _, err := db.Exec(uploadFile, sql.Named("account", account), sql.Named("filename", filename)); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}
			// 移除暫存
			os.Remove(fmt.Sprintf("%s%s", cloudimage.DirPath, filename))
		}

	default:
		http.Error(w, "Invalid API", http.StatusBadRequest)
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
