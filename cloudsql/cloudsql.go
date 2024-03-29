package cloudsql

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"accessToken"`
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
	Name     string `json:"name"`
	FileSize string `json:"filesize"`
	FileUnit string `json:"fileunit"`
	Created  string `json:"created"`
	Link     string `json:"link"`
	Version  string `json:"version"`
}

// getHandler:
func getHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if err := r.ParseForm(); err != nil {
		log.Printf("GET: failed to parse form: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	accessToken := r.Header.Get("Authorization")
	accessToken = strings.Split(accessToken, " ")[1]
	account, err := authToken(accessToken)
	if err != nil {
		http.Error(w, "Invalid Access Token", http.StatusForbidden)
		return
	}
	queryName := strings.Split(r.RequestURI, "?")[0]
	switch queryName {
	case "/api/list":
		listQuery := "exec ListImage @account"
		rows, err := db.Query(listQuery, sql.Named("account", account))
		if err != nil {
			log.Printf("Error: unable get image list: %v", err.Error())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		var result []Image
		for rows.Next() {
			var img Image
			if err := rows.Scan(&img.Name, &img.FileSize, &img.FileUnit, &img.Created, &img.Link, &img.Version); err != nil {
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
			if err := cloudstorage.DownloadFile(w, filename, "download"); err != nil {
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
		log.Printf("Post: failed to parse form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	queryName := strings.Split(r.RequestURI, "?")[0]
	switch queryName {
	case "/api/signUp":
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Account == "" || req.Username == "" || req.Password == "" {
			log.Printf("Add member error")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		password, err := cloudkey.SignMac(w, req.Password)
		if err != nil {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		// [START cloud_sql_sqlserver_databasesql_connection]
		addUser := "INSERT INTO Members (account, username, userpassword, createdTime) VALUES (@account, @username, @password, GETDATE())"
		if _, err = db.Exec(addUser, sql.Named("account", req.Account), sql.Named("username", req.Username), sql.Named("password", password)); err != nil {
			log.Printf("Error: unable to add user: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		// [END cloud_sql_sqlserver_databasesql_connection]
		fmt.Fprintf(w, "Member successfully add: %s!", req.Username)

	case "/api/login":
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Account == "" || req.Password == "" {
			log.Printf("Account or Password should not be empty.")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		password, err := cloudkey.SignMac(w, req.Password)
		if err != nil {
			log.Println(err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		verifyUser := "EXEC dbo.VerifyUser @account, @password"

		var mid int
		var username string
		resp := LoginResponse{AccessToken: ""}
		w.Header().Set("Content-Type", "application/json")

		if err := db.QueryRow(verifyUser, sql.Named("account", req.Account), sql.Named("password", password)).Scan(&mid, &username); err != nil {
			log.Printf("Error: unable to login: %v", err)
			json.NewEncoder(w).Encode(resp)
			return
		}
		accessToken, err := cloudkey.CreateToken(mid, req.Account, username)
		if err != nil {
			log.Println(err.Error())
			json.NewEncoder(w).Encode(resp)
			return
		}
		resp = LoginResponse{AccessToken: accessToken}
		json.NewEncoder(w).Encode(resp)
	case "/api/upload":
		accessToken := r.Header.Get("Authorization")
		accessToken = strings.Split(accessToken, " ")[1]
		account, err := authToken(accessToken)
		if err != nil {
			http.Error(w, "Invalid Access Token", http.StatusForbidden)
			return
		}
		switch queryName {
		case "/api/upload":
			// 取得檔案
			file, header, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}
			defer file.Close()

			if _, err := os.Stat(cloudimage.DirPath); os.IsNotExist(err) {
				err = os.Mkdir(cloudimage.DirPath, 0755)
				if err != nil {
					return
				}
			}

			// 儲存檔案
			filename := header.Filename
			var version int

			checkVersion := "select count(*)+1 from images i , members m where m.account = @account and i.Name = @filename and i.mid = m.mid"
			if err := db.QueryRow(checkVersion, sql.Named("account", account), sql.Named("filename", filename)).Scan(&version); err != nil {
				return
			}
			tmp := strings.Split(header.Filename, ".")
			linkName := fmt.Sprintf("%s_v%d.%s", tmp[0], version, tmp[1])
			out, err := os.Create(fmt.Sprintf("%s%s", cloudimage.DirPath, linkName))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println(err.Error())
				return
			}
			defer out.Close()

			if _, err := io.Copy(out, file); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println(err.Error())
				return
			}
			tmpFile := fmt.Sprintf("%s%s", cloudimage.DirPath, linkName)
			fileInfo, err := os.Stat(tmpFile)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			fileSize := float64(fileInfo.Size()) / 1024.0 / 1024.0
			var sizeUnit string
			if fileSize < 1.0 {
				fileSize *= 1024.0
				sizeUnit = "KB"
			} else {
				sizeUnit = "MB"
			}
			fileSizeStr := fmt.Sprintf("%.2f", fileSize)

			// 上傳
			if err := cloudstorage.UploadFile(w, account, linkName); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}

			// 上傳成功記錄 DB
			uploadFile := "exec dbo.InsertImage @account, @filename, @fileSize, @sizeUnit, @linkname, @version"
			if _, err := db.Exec(uploadFile, sql.Named("account", account), sql.Named("filename", filename), sql.Named("fileSize", fileSizeStr), sql.Named("sizeUnit", sizeUnit), sql.Named("linkName", linkName), sql.Named("version", version)); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				log.Println(err.Error())
				return
			}

			// 移除暫存
			os.Remove(tmpFile)

			// 回傳成功訊息
			w.WriteHeader(http.StatusOK)
		}

	default:
		http.Error(w, "Invalid API", http.StatusBadRequest)
	}
}

// authToken :
func authToken(accessToken string) (string, error) {
	claim, err := cloudkey.ValidateToken(accessToken)
	if err != nil {
		log.Println(err.Error())
		return "", err
	}
	account, ok := claim["account"].(string)
	if !ok {
		return "", errors.New("parse access token error")
	}
	return account, nil
}
