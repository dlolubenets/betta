package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

const defaultEmail = "default_player@gmail.com"
const alreadyExistsPostgresCode = "23505"

func main() {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@database/betta?sslmode=disable")
	if err != nil {
		log.Fatalf("failed to connect to the database: %s", err)
	}

	r := mux.NewRouter()
	tc := NewTransactionController(db)
	r.HandleFunc("/transaction", tc.Handle).Methods(http.MethodPost)
	log.Println("Server is ready to accept connections...")

	go func() {
		ticker := time.NewTicker(time.Second * 10)
		for range ticker.C {
			err := PostProcess(db)
			if err != nil {
				log.Println(err)
			}
		}
	}()

	err = http.ListenAndServe("0.0.0.0:8083", r)
	if err != nil {
		log.Fatal(err)
	}
}

type TransactionController struct {
	db *sql.DB
}

func NewTransactionController(db *sql.DB) *TransactionController {
	return &TransactionController{db: db}
}

type TransactionPayload struct {
	State         string
	Amount        string
	TransactionID string
}

type User struct {
	ID      int
	Balance int
	Email   string
}

type Transaction struct {
	ExternalID string
	UserID     int
	Type       string
	Amount     int
	SourceType int
	Processed  bool
	CreatedAt  time.Time
}

func (tc *TransactionController) Handle(rw http.ResponseWriter, r *http.Request) {
	var transactionPayload TransactionPayload
	err := json.NewDecoder(r.Body).Decode(&transactionPayload)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := tc.db.BeginTx(r.Context(), &sql.TxOptions{})
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var sourceTypeID int
	row := tx.QueryRow("SELECT id FROM source_types WHERE value = $1", r.Header.Get("Source-Type"))
	err = row.Scan(&sourceTypeID)
	if err != nil {
		if err == sql.ErrNoRows {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	var user User
	row = tx.QueryRow("SELECT  * FROM users  WHERE email=$1 FOR UPDATE", defaultEmail)
	err = row.Scan(&user.ID, &user.Balance, &user.Email)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	amountF, err := strconv.ParseFloat(transactionPayload.Amount, 64)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	amount := int64(amountF * 1000)

	_, err = tx.Exec(`INSERT INTO transactions (external_id, user_id, type, amount, source_type) 
		VALUES ($1, $2, $3, $4, $5)`,
		transactionPayload.TransactionID, user.ID, transactionPayload.State, amount, sourceTypeID)
	if err != nil {
		pqErr, ok := err.(*pq.Error)
		// if transaction is already in place we quietly proceed with 200
		if ok && pqErr.Code == alreadyExistsPostgresCode {
			rw.WriteHeader(http.StatusOK)
			return
		}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if transactionPayload.State == "win" {
		user.Balance += int(amount)
	} else if transactionPayload.State == "lost" {
		user.Balance -= int(amount)
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if user.Balance < 0 {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = tx.Exec(`UPDATE users SET balance = $1 WHERE id = $2`, user.Balance, user.ID)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = tx.Commit()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

}

func PostProcess(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var user User
	row := tx.QueryRow("SELECT  * FROM users  WHERE email=$1 FOR UPDATE", defaultEmail)
	err = row.Scan(&user.ID, &user.Balance, &user.Email)
	if err != nil {
		return fmt.Errorf("query users: %w", err)
	}

	defer tx.Rollback()
	rows, err := tx.Query("SELECT * FROM transactions WHERE processed=false ORDER BY created_at DESC LIMIT 10")
	if err != nil {
		return fmt.Errorf("query transactions: %w", err)
	}
	var balanceDelta int
	var transactions []Transaction
	for rows.Next() {
		var transaction Transaction
		err := rows.Scan(&transaction.ExternalID, &transaction.UserID, &transaction.Type, &transaction.Amount, &transaction.SourceType, &transaction.Processed, &transaction.CreatedAt)
		if err != nil {
			return err
		}
		if transaction.Type == "win" {
			balanceDelta -= transaction.Amount
		} else if transaction.Type == "lost" {
			balanceDelta += transaction.Amount
		}
		transactions = append(transactions, transaction)
	}

	for _, t := range transactions {
		_, err = tx.Exec("UPDATE transactions SET processed = $1 WHERE external_id = $2", true, t.ExternalID)
		if err != nil {
			return fmt.Errorf("update transactions processed: %w", err)
		}
	}
	user.Balance += balanceDelta
	_, err = tx.Exec(`UPDATE users SET balance = $1 WHERE id = $2`, user.Balance, user.ID)
	if err != nil {
		return fmt.Errorf("update user balance: %w", err)
	}
	return tx.Commit()
}
