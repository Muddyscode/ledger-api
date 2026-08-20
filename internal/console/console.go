package console

import (
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Muddyscode/ledger-api/internal/auth"
	"github.com/Muddyscode/ledger-api/internal/config"
	"github.com/Muddyscode/ledger-api/internal/httpserver"
	"github.com/Muddyscode/ledger-api/internal/ledger"
	"github.com/Muddyscode/ledger-api/internal/money"
	"github.com/Muddyscode/ledger-api/internal/store"
	"github.com/Muddyscode/ledger-api/web"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Console struct {
	cfg   config.Config
	store *store.Store
	tpl   *template.Template
}

func Mount(r chi.Router, cfg config.Config, st *store.Store) error {
	sub, err := fs.Sub(web.FS, "templates")
	if err != nil {
		return err
	}
	tpl, err := template.ParseFS(sub, "*.html")
	if err != nil {
		return err
	}
	c := &Console{cfg: cfg, store: st, tpl: tpl}

	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		return err
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	r.Get("/login", c.loginGET)
	r.Post("/login", c.loginPOST)
	r.Post("/logout", c.logout)

	r.Group(func(r chi.Router) {
		r.Use(c.requireLogin)
		r.Get("/", c.home)
		r.Get("/accounts/{id}", c.account)
		r.Get("/journals", c.journals)
		r.Post("/journals/{id}/reverse", c.reverse)
		r.Get("/move", c.moveGET)
		r.Post("/move", c.movePOST)
	})
	return nil
}

func (c *Console) poster() ledger.StorePoster {
	return ledger.StorePoster{
		LockAccounts:  c.store.LockAccounts,
		InsertJournal: c.store.InsertJournal,
		InsertPosting: c.store.InsertPosting,
		UpdateBalance: c.store.UpdateBalance,
	}
}

func (c *Console) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ledger_token")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		id, _, err := auth.ParseToken(c.cfg.JWTSecret, cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		op, err := c.store.GetOperatorByID(r.Context(), c.store.Pool, id)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r.WithContext(httpserver.WithOperator(r.Context(), op)))
	})
}

func (c *Console) loginGET(w http.ResponseWriter, r *http.Request) {
	_ = c.tpl.ExecuteTemplate(w, "login", map[string]any{
		"Email": c.cfg.SeedEmail,
		"Error": r.URL.Query().Get("error"),
	})
}

func (c *Console) loginPOST(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	pass := r.FormValue("password")
	op, err := c.store.GetOperatorByEmail(r.Context(), c.store.Pool, email)
	if err != nil || !auth.VerifyPassword(pass, op.PasswordHash) {
		http.Redirect(w, r, "/login?error=invalid+credentials", http.StatusFound)
		return
	}
	token, _, err := auth.IssueToken(c.cfg.JWTSecret, op.Email, op.ID, 8*time.Hour)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "ledger_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((8 * time.Hour).Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (c *Console) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "ledger_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (c *Console) home(w http.ResponseWriter, r *http.Request) {
	op := httpserver.OperatorFrom(r)
	accts, err := c.store.ListAccounts(r.Context(), c.store.Pool, op.ID, "")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	idn, err := c.store.Identity(r.Context(), c.store.Pool, op.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	type row struct {
		ledger.Account
		BalanceFmt string
	}
	rows := make([]row, 0, len(accts))
	for _, a := range accts {
		rows = append(rows, row{Account: a, BalanceFmt: money.FormatNGN(a.BalanceMinor)})
	}
	_ = c.tpl.ExecuteTemplate(w, "home", map[string]any{
		"Email":    op.Email,
		"Accounts": rows,
		"Holds":    idn.IdentityMinor == 0 && idn.CacheDrift == 0,
		"Identity": idn.IdentityMinor,
	})
}

func (c *Console) account(w http.ResponseWriter, r *http.Request) {
	op := httpserver.OperatorFrom(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	acct, err := c.store.GetAccount(r.Context(), c.store.Pool, op.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	entries, _, err := c.store.Statement(r.Context(), c.store.Pool, op.ID, id, 200)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	type line struct {
		CreatedAt   string
		JournalID   string
		Direction   string
		AmountMinor int64
		Running     int64
	}
	var ls []line
	for _, e := range entries {
		ls = append(ls, line{
			CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
			JournalID:   e.Posting.JournalID.String()[:8],
			Direction:   string(e.Posting.Direction),
			AmountMinor: e.Posting.Amount,
			Running:     e.Running,
		})
	}
	_ = c.tpl.ExecuteTemplate(w, "account", map[string]any{
		"Email":      op.Email,
		"Account":    acct,
		"BalanceFmt": money.FormatNGN(acct.BalanceMinor),
		"Entries":    ls,
	})
}

func (c *Console) journals(w http.ResponseWriter, r *http.Request) {
	op := httpserver.OperatorFrom(r)
	js, err := c.store.ListJournals(r.Context(), c.store.Pool, op.ID, 50, false)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	type pj struct {
		AccountID string
		Direction string
		Amount    int64
	}
	type jv struct {
		ID          string
		CreatedAt   string
		Description string
		Reverses    string
		Postings    []pj
	}
	var out []jv
	for _, j := range js {
		rev := ""
		if j.ReversesJournalID != nil {
			rev = j.ReversesJournalID.String()[:8]
		}
		item := jv{
			ID:          j.ID.String(),
			CreatedAt:   j.CreatedAt.UTC().Format(time.RFC3339),
			Description: j.Description,
			Reverses:    rev,
		}
		for _, p := range j.Postings {
			item.Postings = append(item.Postings, pj{AccountID: p.AccountID.String()[:8], Direction: string(p.Direction), Amount: p.Amount})
		}
		out = append(out, item)
	}
	_ = c.tpl.ExecuteTemplate(w, "journals", map[string]any{"Email": op.Email, "Journals": out})
}

func (c *Console) reverse(w http.ResponseWriter, r *http.Request) {
	op := httpserver.OperatorFrom(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	orig, err := c.store.GetJournal(r.Context(), c.store.Pool, op.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = store.RetryDeadlock(func() error {
		return c.store.WithTx(r.Context(), func(tx pgx.Tx) error {
			_, err := ledger.Post(r.Context(), tx, c.poster(), ledger.PostRequest{
				OperatorID:        op.ID,
				Description:       "reversal of " + orig.ID.String(),
				Lines:             ledger.ReverseLines(orig.Postings),
				ReversesJournalID: &orig.ID,
			})
			return err
		})
	})
	if err != nil {
		http.Redirect(w, r, "/journals?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/journals", http.StatusFound)
}

func (c *Console) moveGET(w http.ResponseWriter, r *http.Request) {
	op := httpserver.OperatorFrom(r)
	accts, err := c.store.ListAccounts(r.Context(), c.store.Pool, op.ID, "")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = c.tpl.ExecuteTemplate(w, "move", map[string]any{
		"Email":    op.Email,
		"Accounts": accts,
		"Error":    r.URL.Query().Get("error"),
		"OK":       r.URL.Query().Get("ok"),
	})
}

func (c *Console) movePOST(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	op := httpserver.OperatorFrom(r)
	recipe := r.FormValue("recipe")
	amount, err := money.ParseJSONAmount([]byte(strings.TrimSpace(r.FormValue("amount"))))
	if err != nil {
		http.Redirect(w, r, "/move?error="+url.QueryEscape("amount must be integer kobo"), http.StatusFound)
		return
	}
	lookup := func(code string) (uuid.UUID, error) {
		a, err := c.store.GetAccountByCode(r.Context(), c.store.Pool, op.ID, code)
		return a.ID, err
	}
	var debit, credit uuid.UUID
	switch recipe {
	case "p2p":
		debit, err = lookup("2000")
		if err == nil {
			credit, err = lookup("2010")
		}
	case "deposit":
		debit, err = lookup("1000")
		if err == nil {
			credit, err = lookup("2000")
		}
	case "withdraw":
		debit, err = lookup("2000")
		if err == nil {
			credit, err = lookup("1000")
		}
	case "fee":
		debit, err = lookup("2000")
		if err == nil {
			credit, err = lookup("4000")
		}
	case "expense":
		debit, err = lookup("5000")
		if err == nil {
			credit, err = lookup("1000")
		}
	default:
		debit, err = uuid.Parse(r.FormValue("debit"))
		if err == nil {
			credit, err = uuid.Parse(r.FormValue("credit"))
		}
	}
	if err != nil {
		http.Redirect(w, r, "/move?error="+url.QueryEscape("unknown accounts"), http.StatusFound)
		return
	}
	desc := strings.TrimSpace(r.FormValue("description"))
	if desc == "" {
		desc = recipe
	}
	err = store.RetryDeadlock(func() error {
		return c.store.WithTx(r.Context(), func(tx pgx.Tx) error {
			_, err := ledger.Post(r.Context(), tx, c.poster(), ledger.PostRequest{
				OperatorID:           op.ID,
				Description:          desc,
				EnforceTransferPairs: true,
				Lines: []ledger.Line{
					{AccountID: debit, Direction: ledger.Debit, Amount: amount},
					{AccountID: credit, Direction: ledger.Credit, Amount: amount},
				},
			})
			return err
		})
	})
	if err != nil {
		http.Redirect(w, r, "/move?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/move?ok=posted", http.StatusFound)
}
