package main

import (
	"database/sql"  // DB, ErrNoRows, Open
	"encoding/json" // Marshal, Unmarshal
	"html"          // EscapeString
	"html/template" // HTML
	"io"            // EOF
	"io/ioutil"     // ReadAll
	"log"           // Fatal, Println
	"net/http"
	"strconv" // FormatUint, ParseUint
	"sync"    // WaitGroup
	"time"    // Hour, Now, Time

	"github.com/emersion/go-imap"        // MailboxInfo, Message
	"github.com/emersion/go-imap/client" // Client, DialTLS
	"github.com/emersion/go-message/mail"
	"golang.org/x/crypto/bcrypt" // CompareHashAndPassword
)

type MailAccount struct {
	Server   string
	Username string
	Password string
}

type MailAccountNoPassword struct {
	Server   string
	Username string
}

type PageData struct {
	Server        string
	Username      string
	Mbox          string
	Previews      []*MailPreviewData
	OtherMboxes   []string
	OtherAccounts []*AccountData
}

type AccountData struct {
	Server   string
	Username string
	Mboxes   []string
}

type MboxData struct {
	Server   string
	Username string
	Mbox     string
	Previews []*MailPreviewData
}

type MailData struct {
	Subject string
	RawBody template.HTML
}

type MailPreviewData struct {
	Date    string
	Subject string
	Preview string
	Uid     string
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		http.ServeFile(w, r, "signin.html")
		return
	}

	accounts := accounts(username)
	if len(accounts) < 1 {
		http.ServeFile(w, r, "onboard.html")
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(accounts))
	mboxes := []string{}
	previews := []*MailPreviewData{}
	otherAccounts := make([]*AccountData, len(accounts)-1)

	go func() {
		c, err := newClient(accounts[0])
		defer c.Logout()
		if err != nil {
			log.Fatal(err)
		}

		mboxes = fetchMboxes(c)
		cutoff := time.Now().Add(-24 * time.Hour)
		for _, m := range fetchMessages(c, mboxes[0]) {
			previews = append(previews, newMailPreviewData(m, cutoff))
		}
		wg.Done()
	}()

	for i, a := range accounts[1:] {
		go func(a *MailAccount, i int) {
			c, err := newClient(a)
			defer c.Logout()
			if err != nil {
				log.Fatal(err)
			}
			otherAccounts[i] = &AccountData{a.Server, a.Username, fetchMboxes(c)}
			wg.Done()
		}(a, i)
	}

	wg.Wait()

	_tmpl.Execute(w, PageData{
		Server:        accounts[0].Server,
		Username:      accounts[0].Username,
		Mbox:          mboxes[0],
		Previews:      previews,
		OtherMboxes:   mboxes[1:],
		OtherAccounts: otherAccounts,
	})
}

func handleSignin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.Form.Get("username")
	password := r.Form.Get("password")
	if username == "" || password == "" {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}

	stmt, err := _db.Prepare("select bcrypt_hash from users where username == ?")
	defer stmt.Close()
	if err != nil {
		log.Fatal(err)
	}
	var storedHash string
	err = stmt.QueryRow(username).Scan(&storedHash)
	if err == sql.ErrNoRows {
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Fatal(err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	if err != nil {
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
		return
	}

	session, _ := _cookies.Get(r, "session")
	session.Values["username"] = username
	err = session.Save(r, w)
	if err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleSignout(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	session.Options.MaxAge = -1
	err := session.Save(r, w)
	if err != nil {
		log.Fatal(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleMboxMain(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	db := db(username)
	defer db.Close()
	var server, mailname, password string
	err := db.QueryRow(`select * from accounts limit 1`).Scan(&server, &mailname, &password)
	if err != nil {
		log.Fatal(err)
	}
	a := &MailAccount{server, mailname, password}

	c, err := newClient(a)
	defer c.Logout()
	if err != nil {
		log.Fatal(err)
	}

	mboxes := fetchMboxes(c)
	sendMessagesJSON(c, server, mailname, mboxes[0], w)
}

func handleMboxName(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// TODO: Sync token
	server := r.URL.Query().Get("server")
	mailname := r.URL.Query().Get("username")
	mboxName := r.URL.Query().Get("mboxName")
	if server == "" || mailname == "" || mboxName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	a := account(username, server, mailname)
	if a == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c, err := newClient(a)
	defer c.Logout()
	if err != nil {
		log.Fatal(err)
	}
	sendMessagesJSON(c, server, mailname, mboxName, w)
}

func handleMail(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	server := r.URL.Query().Get("server")
	mailname := r.URL.Query().Get("username")
	mboxName := r.URL.Query().Get("mboxName")
	uid := r.URL.Query().Get("uid")
	if server == "" || mailname == "" || mboxName == "" || uid == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	a := account(username, server, mailname)
	if a == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c, err := newClient(a)
	defer c.Logout()
	if err != nil {
		log.Fatal(err)
	}

	uid64, err := strconv.ParseUint(uid, 10, 32)
	if err != nil {
		log.Fatal(err)
	}
	m := newMailData(fetchMessage(c, mboxName, uint32(uid64)))

	j, err := json.Marshal(m)
	if err != nil {
		log.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}

func handleOnboard(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		http.Redirect(w, r, "/", http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	var accounts []*MailAccount
	err = json.Unmarshal(body, &accounts)
	if err != nil {
		log.Fatal(err)
	}

	db := db(username)
	defer db.Close()
	stmt, err := db.Prepare(
		`insert or replace into accounts (
	server, username, password
) values (
	?, ?, ?
)`)
	if err != nil {
		log.Fatal(err)
	}

	for _, a := range accounts {
		_, err := stmt.Exec(a.Server, a.Username, a.Password)
		if err != nil {
			log.Fatal(err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func handleAddAccount(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	var a *MailAccount
	err = json.Unmarshal(body, &a)
	if err != nil {
		panic(err) //*
	}
	if a.Server == "" || a.Username == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	db := db(username)
	defer db.Close()
	stmt, err := db.Prepare(
		`insert or replace into accounts (
	server, username, password
) values (
	?, ?, ?
)`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt.Exec(a.Server, a.Username, a.Password)
	if err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
	log.Println("AddAccount: ", a) //*
}

func handleRemoveAccount(w http.ResponseWriter, r *http.Request) {
	session, _ := _cookies.Get(r, "session")
	username, ok := session.Values["username"].(string)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}
	var anp *MailAccountNoPassword
	err = json.Unmarshal(body, &anp)
	if err != nil {
		panic(err) //*
	}

	db := db(username)
	defer db.Close()
	stmt, err := db.Prepare(`delete from accounts where server = ? and username = ?`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt.Exec(anp.Server, anp.Username)
	if err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusOK)
	log.Println("RemoveAccount: ", anp) //*
}

func accounts(username string) []*MailAccount {
	db := db(username)
	defer db.Close()
	rows, err := db.Query("select * from accounts")
	defer rows.Close()
	if err != nil {
		log.Fatal(err)
	}
	accounts := []*MailAccount{}
	for rows.Next() {
		var server, mailname, password string
		err = rows.Scan(&server, &mailname, &password)
		if err != nil {
			log.Fatal(err)
		}
		accounts = append(accounts, &MailAccount{server, mailname, password})
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return accounts
}

func account(username, server, mailname string) *MailAccount {
	db := db(username)
	defer db.Close()
	stmt, err := db.Prepare("select password from accounts where server == ? and username == ?")
	defer stmt.Close()
	if err != nil {
		log.Fatal(err)
	}
	var password string
	err = stmt.QueryRow(server, mailname).Scan(&password)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		log.Fatal(err)
	}
	return &MailAccount{server, mailname, password}
}

func db(username string) *sql.DB {
	stmt, err := _db.Prepare("select db_path from users where username == ?")
	defer stmt.Close()
	if err != nil {
		log.Fatal(err)
	}
	var dbPath string
	err = stmt.QueryRow(username).Scan(&dbPath)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func sendMessagesJSON(c *client.Client, server, username, mboxName string, w http.ResponseWriter) {
	previews := []*MailPreviewData{}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, m := range fetchMessages(c, mboxName) {
		previews = append(previews, newMailPreviewData(m, cutoff))
	}

	mboxData := &MboxData{server, username, mboxName, previews}
	j, err := json.Marshal(mboxData)
	if err != nil {
		log.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}

func newClient(a *MailAccount) (*client.Client, error) {
	log.Println("DialTLS " + a.Server)
	c, err := client.DialTLS(a.Server, nil)
	if err != nil {
		return nil, err
	}
	err = c.Login(a.Username, a.Password)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func newMailData(m *imap.Message) *MailData {
	return &MailData{subject(m), rawbody(m)}
}

func newMailPreviewData(m *imap.Message, cutoff time.Time) *MailPreviewData {
	return &MailPreviewData{
		Date:    prettyTime(m.Envelope.Date, cutoff),
		Subject: subject(m),
		Preview: preview(m),
		Uid:     strconv.FormatUint(uint64(m.Uid), 10),
	}
}

func prettyTime(t time.Time, cutoff time.Time) string {
	if t.After(cutoff) {
		return t.Format("3:04 PM")
	} else {
		return t.Format("Mon Jan _2 2006")
	}
}

func subject(m *imap.Message) string {
	sub := m.Envelope.Subject
	if len(sub) < 1 {
		return "No Subject"
	}
	return sub
}

func preview(m *imap.Message) string {
	body := m.GetBody(&imap.BodySectionName{})
	if body == nil {
		return "Empty"
	} else if r, err := mail.CreateReader(body); err != nil {
		return err.Error()
	} else {
		var preview string
		for {
			part, err := r.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				preview += err.Error()
				break
			}

			data, err := ioutil.ReadAll(part.Body)
			if err != nil {
				preview += err.Error()
			}
			switch h := part.Header.(type) {
			case mail.TextHeader:
				ct, _, _ := h.ContentType()
				switch ct {
				case "text/plain":
					preview += html.EscapeString(string(data))
				}
			case mail.AttachmentHeader:
				filename, err := h.Filename()
				if err != nil {
					preview += err.Error()
				} else {
					preview += filename
				}
			}
		}
		r.Close()
		return preview
	}
}

func rawbody(m *imap.Message) template.HTML {
	body := m.GetBody(&imap.BodySectionName{})
	if body == nil {
		return ""
	} else if r, err := mail.CreateReader(body); err != nil {
		return template.HTML(err.Error())
	} else {
		preferHTML := false
		renderers := []Renderer{}
		for {
			part, err := r.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				renderers = append(renderers, TextRenderer(err.Error()))
				break
			}

			bytes, err := ioutil.ReadAll(part.Body)
			if err != nil {
				renderers = append(renderers, TextRenderer(err.Error()))
				break
			}

			switch h := part.Header.(type) {
			case mail.TextHeader:
				ct, _, _ := h.ContentType()
				switch ct {
				case "text/plain":
					renderers = append(renderers, TextRenderer(bytes))
				case "text/html":
					preferHTML = true
					// TODO: Sanitize scripts and remote content
					// - iframe sandbox?
					renderers = append(renderers, HTMLRenderer(bytes))
				default:
					log.Println("Unknown Content-Type " + ct)
				}
			case mail.AttachmentHeader:
				filename, err := h.Filename()
				if err != nil {
					renderers = append(renderers, TextRenderer(err.Error()))
				} else {
					renderers = append(renderers, AttachmentRenderer{bytes, filename})
				}
			}
		}
		r.Close()

		var html template.HTML
		for _, r := range renderers {
			if preferHTML {
				if _, ok := r.(TextRenderer); ok {
					continue
				}
			}
			html += template.HTML(r.render())
		}
		return html
	}
}
