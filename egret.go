package main

import (
	"bufio"         // NewReader
	"database/sql"  // DB, Open
	"flag"          // Bool, Parse, String
	"fmt"           // Print, Println
	"html/template" // Must, ParseFiles
	"log"           // Fatal, Println
	"net/http"      // Dir, FileServer, Server
	"os"            // IsNotExist, Stat, Stdin
	"strings"       // TrimSuffix
	"syscall"       // Stdin
	"time"          // Second

	"github.com/gorilla/mux"          // NewRouter
	"github.com/gorilla/securecookie" // GenerateRandomKey
	"github.com/gorilla/sessions"     // CookieStore, NewCookieStore
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"       // DefaultCost, GenerateFromPassword
	"golang.org/x/crypto/ssh/terminal" // ReadPassword
)

var (
	_cookies *sessions.CookieStore
	_db      *sql.DB
	_tmpl    = template.Must(template.ParseFiles("main.tmpl"))
)

func main() {
	var err error

	flgAddUsers := flag.Bool("add-users", false, "Add users")
	flgPort := flag.String("port", "80", "HTTP port")
	flgDBPath := flag.String("db", "./egret.db", "Database path")
	flag.Parse()

	_db, err = sql.Open("sqlite3", *flgDBPath)
	if err != nil {
		log.Fatal(err)
	}
	_, err = _db.Exec(
		`create table if not exists users (
  username text not null primary key,
  bcrypt_hash text not null,
  db_path text not null
);`)
	if err != nil {
		log.Fatal(err)
	}

	if *flgAddUsers {
		addUsers()
	}

	authKey := securecookie.GenerateRandomKey(64)
	encryptionKey := securecookie.GenerateRandomKey(32)
	_cookies = sessions.NewCookieStore(authKey, encryptionKey)

	r := mux.NewRouter()
	r.HandleFunc("/", handleIndex)
	r.HandleFunc("/signin", handleSignin)
	r.HandleFunc("/signout", handleSignout)
	r.HandleFunc("/mboxMain", handleMboxMain)
	r.HandleFunc("/mboxName", handleMboxName)
	r.HandleFunc("/mail", handleMail)
	r.HandleFunc("/onboard", handleOnboard)
	r.HandleFunc("/addAccount", handleAddAccount)
	r.HandleFunc("/removeAccount", handleRemoveAccount)
	r.PathPrefix("/js").Handler(http.FileServer(http.Dir(".")))

	log.Println("Listening...")
	s := http.Server{
		Addr:         ":" + (*flgPort),
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	err = s.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func addUsers() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("--- Add User ---")

		var err error
		var username string
		var password []byte
		var dbPath string
		for {
			fmt.Print("Username: ")
			username, err = reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			username = strings.TrimSuffix(username, "\n")
			if username != "" {
				break
			}
		}
		for {
			fmt.Print("Password: ")
			password, err = terminal.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				log.Fatal(err)
			}
			if len(password) > 0 {
				break
			}
		}
		for {
			fmt.Print("Database path: ")
			dbPath, err = reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			dbPath = strings.TrimSuffix(dbPath, "\n")
			if dbPath != "" {
				_, err = os.Stat(dbPath)
				if err != nil && os.IsNotExist(err) {
					break
				}
			}
		}

		hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
		if err != nil {
			log.Fatal(err)
		}

		_, err = _db.Exec(
			`insert or replace into users (
	username, bcrypt_hash, db_path
) values (
	?, ?, ?
)`,
			username, hash, dbPath,
		)
		if err != nil {
			log.Fatal(err)
		}

		userDB, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatal(err)
		}
		_, err = userDB.Exec(
			`create table accounts (
	server text not null,
	username text not null,
	password text not null,
	primary key (server, username)
)`)

		fmt.Println("--- ******** ---")

		var addAnother bool
		for {
			fmt.Print("Add another user? (y/N): ")
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			input = strings.TrimSuffix(input, "\n")
			if input == "y" || input == "Y" {
				addAnother = true
				break
			} else if input == "" || input == "n" || input == "N" {
				addAnother = false
				break
			}
		}
		if !addAnother {
			break
		}
	}
}
