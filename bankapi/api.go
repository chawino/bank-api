package bankapi

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

var ErrNotFound = errors.New("user: not found")

type User struct {
	mu        sync.Mutex
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name" binding:"required"`
	LastName  string    `json:"last_name" binding:"required"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type BankAccount struct {
	mu            sync.Mutex
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	AccountNumber string    `json:"account_number" binding:"required"`
	Name          string    `json:"name"`
	Balance       int       `json:"balance"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Secret struct {
	ID  int64  `json:"id"`
	Key string `json:"key" binding:"required"`
}

type Server struct {
	db                 *sql.DB
	userService        UserService
	bankAccountService BankAccountService
	transferService    TransferService
	secretService      SecretService
}

type UserService interface {
	All() ([]User, error)
	Insert(user *User) error
	InsertBankAccount(bankAccount *BankAccount) error
	GetByID(id int) (*User, error)
	GetBankAccountsByUserID(user int) ([]BankAccount, error)
	Update(id int, first_name string, last_name string) (*User, error)
	DeleteByID(id int) error
}

type BankAccountService interface {
	Deposit(bankAccountId int, balance int) (*BankAccount, error)
	Withdraw(bankAccountId int, balance int) (*BankAccount, error)
	DeleteAccountByBankAccountID(bankAccountId int) error
}

type TransferService interface {
	Transfer(from string, to string, amount int) error
}

type SecretService interface {
	Insert(s *Secret) error
}

type SecretServiceImp struct {
	db *sql.DB
}

func (s *Server) CreateSecret(c *gin.Context) {
	var secret Secret
	if err := c.ShouldBindJSON(&secret); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}
	if err := s.secretService.Insert(&secret); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, secret)
}

func (s *SecretServiceImp) Insert(secret *Secret) error {
	row := s.db.QueryRow("INSERT INTO secrets (key) values ($1) RETURNING id", secret.Key)

	if err := row.Scan(&secret.ID); err != nil {
		return err
	}
	return nil
}

type UserServiceImp struct {
	mu sync.Mutex
	db *sql.DB
}

type BankAccountServiceImp struct {
	mu sync.Mutex
	db *sql.DB
}

type TransferServiceImp struct {
	mu sync.Mutex
	db *sql.DB
}

func (s *UserServiceImp) All() ([]User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query("SELECT * FROM users")
	if err != nil {
		return nil, err
	}
	users := []User{} // set empty slice without nil
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.UpdatedAt, &user.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *UserServiceImp) Insert(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	row := s.db.QueryRow("INSERT INTO users (first_name, last_name, created_at, updated_at) values ($1, $2, $3, $4) RETURNING id", user.FirstName, user.LastName, now, now)

	if err := row.Scan(&user.ID); err != nil {
		return err
	}
	return nil
}

func (s *UserServiceImp) GetByID(id int) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt := "SELECT * FROM users WHERE id = $1"
	row := s.db.QueryRow(stmt, id)
	var user User
	err := row.Scan(&user.ID, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserServiceImp) Update(id int, fisrt_name string, last_name string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt := "UPDATE users SET first_name = $2, last_name = $3 WHERE id = $1"
	_, err := s.db.Exec(stmt, id, fisrt_name, last_name)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *UserServiceImp) DeleteByID(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt := "DELETE FROM users WHERE id = $1"
	_, err := s.db.Exec(stmt, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) All(c *gin.Context) {
	todos, err := s.userService.All()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"object":  "error",
			"message": fmt.Sprintf("db: query error: %s", err),
		})
		return
	}
	c.JSON(http.StatusOK, todos)
}

func (s *Server) Create(c *gin.Context) {
	var user User
	err := c.ShouldBindJSON(&user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"object":  "error",
			"message": fmt.Sprintf("json: wrong params: %s", err),
		})
		return
	}

	if err := s.userService.Insert(&user); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (s *Server) GetByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	user, err := s.userService.GetByID(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) Update(c *gin.Context) {
	h := map[string]string{}
	if err := c.ShouldBindJSON(&h); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	todo, err := s.userService.Update(id, h["first_name"], h["last_name"])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, todo)
}

func (s *Server) DeleteByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := s.userService.DeleteByID(id); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
}

// ####### BANK ACCOUNT #########

func (s *UserServiceImp) InsertBankAccount(bankAccount *BankAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// check user_id exist
	user, err := s.GetByID(int(bankAccount.UserID))
	if err != nil {
		return err
	}

	// check account_number exist
	now := time.Now()
	bankAccount.CreatedAt = now
	bankAccount.UpdatedAt = now
	bankAccount.Balance = 0
	row := s.db.QueryRow("INSERT INTO bank_accounts (user_id, account_number, account_name, balance, created_at, updated_at) values ($1, $2, $3, $4, $5, $6) RETURNING id", bankAccount.UserID, bankAccount.AccountNumber, user.FirstName+user.LastName, bankAccount.Balance, now, now)

	if err := row.Scan(&bankAccount.ID); err != nil {
		return err
	}
	return nil
}

func (s *UserServiceImp) GetBankAccountsByUserID(id int) ([]BankAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println("GetBankAccountsByUserId " + strconv.Itoa(id))
	rows, err := s.db.Query("SELECT * FROM bank_accounts WHERE user_id = $1", id)
	if err != nil {
		return nil, err
	}

	//fmt.Println("GetBankAccountsByUserId rows size" + strconv.Itoa(len(rows)))
	bankAccounts := []BankAccount{} // set empty slice without nil
	for rows.Next() {
		var bankAccount BankAccount
		fmt.Println("GetBankAccountsByUserId rows size" + bankAccount.AccountNumber)
		err := rows.Scan(&bankAccount.ID, &bankAccount.UserID, &bankAccount.AccountNumber, &bankAccount.Name, &bankAccount.Balance, &bankAccount.UpdatedAt, &bankAccount.CreatedAt)
		if err != nil {
			fmt.Println("GetBankAccountsByUserId error" + err.Error())
			return nil, err
		}
		bankAccounts = append(bankAccounts, bankAccount)
	}
	return bankAccounts, nil
}

func (s *Server) CreateBankAccount(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	var bankAccount BankAccount
	err := c.ShouldBindJSON(&bankAccount)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"object":  "error",
			"message": fmt.Sprintf("json: wrong params: %s", err),
		})
		return
	}

	bankAccount.UserID = int64(userId)

	if err := s.userService.InsertBankAccount(&bankAccount); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, bankAccount)
}

func (s *Server) GetBankAccountsByUserID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	bankAccounts, err := s.userService.GetBankAccountsByUserID(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, bankAccounts)
}

func (s *Server) DeleteAccountByBankAccountID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := s.bankAccountService.DeleteAccountByBankAccountID(id); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
}

func (s *BankAccountServiceImp) DeleteAccountByBankAccountID(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt := "DELETE FROM bank_accounts WHERE id = $1"
	_, err := s.db.Exec(stmt, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) DepositByID(c *gin.Context) {
	h := map[string]int{}
	if err := c.ShouldBindJSON(&h); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	todo, err := s.bankAccountService.Deposit(id, h["amount"])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, todo)
}

func (s *BankAccountServiceImp) Deposit(id int, amount int) (*BankAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt := "SELECT * FROM bank_accounts WHERE id = $1"
	row := s.db.QueryRow(stmt, id)
	var bankAccount BankAccount
	err := row.Scan(&bankAccount.ID, &bankAccount.UserID, &bankAccount.AccountNumber, &bankAccount.Name, &bankAccount.Balance, &bankAccount.CreatedAt, &bankAccount.UpdatedAt)
	if err != nil {
		return nil, err
	}

	balance := bankAccount.Balance
	b := balance + amount
	bankAccount.Balance = b

	stmt = "UPDATE bank_accounts SET balance = $2 WHERE id = $1"
	_, err = s.db.Exec(stmt, id, b)
	if err != nil {
		return nil, err
	}

	return &bankAccount, nil
}

func (s *Server) GetBankAccountByBankAccountId(id int) (*BankAccount, error) {
	stmt := "SELECT id, user_id, amount FROM bank_accounts WHERE id = $1"
	row := s.db.QueryRow(stmt, id)
	var bankAccount BankAccount
	err := row.Scan(&bankAccount.ID, &bankAccount.UserID, &bankAccount.Balance)
	if err != nil {
		return nil, err
	}
	return &bankAccount, nil
}

func (s *Server) WithdrawByID(c *gin.Context) {
	h := map[string]int{}
	if err := c.ShouldBindJSON(&h); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	todo, err := s.bankAccountService.Withdraw(id, h["amount"])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, todo)
}

func (s *BankAccountServiceImp) Withdraw(id int, amount int) (*BankAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt := "SELECT * FROM bank_accounts WHERE id = $1"
	row := s.db.QueryRow(stmt, id)
	var bankAccount BankAccount
	err := row.Scan(&bankAccount.ID, &bankAccount.UserID, &bankAccount.AccountNumber, &bankAccount.Name, &bankAccount.Balance, &bankAccount.CreatedAt, &bankAccount.UpdatedAt)
	if err != nil {
		return nil, err
	}

	balance := bankAccount.Balance
	b := balance - amount
	bankAccount.Balance = b

	stmt = "UPDATE bank_accounts SET balance = $2 WHERE id = $1"
	_, err = s.db.Exec(stmt, id, b)
	if err != nil {
		return nil, err
	}

	return &bankAccount, nil
}

// #### TRANSFER Service ####

func (s *Server) Transfer(c *gin.Context) {
	h := struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Amount int    `json:"amount"`
	}{}
	if err := c.ShouldBindJSON(&h); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}
	err := s.transferService.Transfer(h.From, h.To, h.Amount)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("%s", err),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message":  "transferred",
	})
}

func (s *TransferServiceImp) Transfer(from string, to string, amount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// query from account
	stmt := "SELECT * FROM bank_accounts WHERE account_number = $1"
	row := s.db.QueryRow(stmt, from)
	var fromAccount BankAccount
	err := row.Scan(&fromAccount.ID, &fromAccount.UserID, &fromAccount.AccountNumber, &fromAccount.Name, &fromAccount.Balance, &fromAccount.CreatedAt, &fromAccount.UpdatedAt)
	if err != nil {
		return err
	}

	// query to account
	stmt = "SELECT * FROM bank_accounts WHERE account_number = $1"
	row = s.db.QueryRow(stmt, to)
	var toAccount BankAccount
	err = row.Scan(&toAccount.ID, &toAccount.UserID, &toAccount.AccountNumber, &toAccount.Name, &toAccount.Balance, &toAccount.CreatedAt, &toAccount.UpdatedAt)
	if err != nil {
		return err
	}

	// check balance from account
	balanceFrom := fromAccount.Balance
	if balanceFrom < amount {
		return errors.New("Balance less than amount")
	}

	// update balance from account before add amount to receiver
	fromAccount.Balance = balanceFrom - amount
	now := time.Now()
	fromAccount.CreatedAt = now
	fromAccount.UpdatedAt = now

	stmt = "UPDATE bank_accounts SET balance = $2 WHERE account_number = $1"
	_, err = s.db.Exec(stmt, from, fromAccount.Balance)
	if err != nil {
		return err
	}

	// update balance to account after ... amount to receiver
	toAccount.Balance = toAccount.Balance + amount
	now = time.Now()
	toAccount.CreatedAt = now
	toAccount.UpdatedAt = now

	stmt = "UPDATE bank_accounts SET balance = $2 WHERE account_number = $1"
	_, err = s.db.Exec(stmt, to, toAccount.Balance)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) AuthTodo(c *gin.Context) {
	user, _, ok := c.Request.BasicAuth()
	if ok {
		row := s.db.QueryRow("SELECT key FROM secrets WHERE key = $1", user)
		if err := row.Scan(&user); err == nil {
			return
		}
	}
	c.AbortWithStatus(http.StatusUnauthorized)
}

func setupRoute(s *Server) *gin.Engine {
	r := gin.New()
	r.Use(RequestLogger())
	users := r.Group("/users")
	bankAccounts := r.Group("/bankAccounts")
	transfers := r.Group("/transfers")
	admin := r.Group("/admin")

	admin.Use(gin.BasicAuth(gin.Accounts{
		"admin": "1234",
	}))
	users.Use(s.AuthTodo)
	users.GET("/", s.All)
	users.POST("/", s.Create)
	users.GET("/:id", s.GetByID)
	users.PUT("/:id", s.Update)
	users.DELETE("/:id", s.DeleteByID)

	users.POST("/:id/bankAccount", s.CreateBankAccount)
	users.GET("/:id/bankAccount", s.GetBankAccountsByUserID)

	bankAccounts.Use(s.AuthTodo)
	bankAccounts.DELETE("/:id", s.DeleteAccountByBankAccountID)
	bankAccounts.PUT("/:id/withdraw", s.WithdrawByID)
	bankAccounts.PUT("/:id/deposit", s.DepositByID)

	transfers.Use(s.AuthTodo)
	transfers.POST("/", s.Transfer)

	admin.POST("/secrets", s.CreateSecret)

	return r
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("####### Print request body #######") // Print request body
		fmt.Println(c.Request)
		fmt.Println("####### END Print request body #######") // Print request body
	}
}

func StartServer() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		return
	}
	createTable := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		first_name TEXT,
		last_name TEXT,
		created_at TIMESTAMP WITHOUT TIME ZONE,
		updated_at TIMESTAMP WITHOUT TIME ZONE
	);
	CREATE TABLE IF NOT EXISTS bank_accounts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		account_number TEXT UNIQUE,
		account_name TEXT,
		balance INTEGER,
		created_at TIMESTAMP WITHOUT TIME ZONE,
		updated_at TIMESTAMP WITHOUT TIME ZONE
	);
	CREATE TABLE IF NOT EXISTS secrets (
		id SERIAL PRIMARY KEY,
		key TEXT
	);
	`

	if _, err := db.Exec(createTable); err != nil {
		fmt.Printf("%s", err)
		return
	}

	s := &Server{
		db: db,
		userService: &UserServiceImp{
			db: db,
		},
		bankAccountService: &BankAccountServiceImp{
			db: db,
		},
		transferService: &TransferServiceImp{
			db: db,
		},
		secretService: &SecretServiceImp{
			db: db,
		},
	}

	r := setupRoute(s)

	r.Run(":" + os.Getenv("PORT"))
}
