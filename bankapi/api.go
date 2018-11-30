package bankapi

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
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
	Name          string    `json:"Name"`
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
}

type UserService interface {
	All() ([]User, error)
	Insert(user *User) error
	InsertBankAccount(userId int, bankAccount *BankAccount) error
	GetByID(id int) (*User, error)
	GetBankAccountByUserId(user int) (*BankAccount, error)
	Update(id int, first_name string, last_name string) (*User, error)
	DeleteByID(id int) error
}

type BankAccountService interface {
	Insert(userId int, bankAccount *BankAccount) error
	GetByID(userId int) (*BankAccount, error)
	//Deposit(userId int, balance int) (*BankAccount, error)
	//Withdraw(userId int, balance int) (*BankAccount, error)
	//DeleteByID(userId int) error
	//Transfer(fromAccountNumber int, tooAccountNumber int, balance int) error
}

type UserServiceImp struct {
	db *sql.DB
}

type BankAccountServiceImp struct {
	db *sql.DB
}

func (s *UserServiceImp) All() ([]User, error) {
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
	stmt := "UPDATE users SET first_name = $2, last_name = $3 WHERE id = $1"
	_, err := s.db.Exec(stmt, id, fisrt_name, last_name)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *UserServiceImp) DeleteByID(id int) error {
	stmt := "DELETE FROM users WHERE id = $1"
	_, err := s.db.Exec(stmt, id)
	if err != nil {
		return err
	}
	return nil
}

func AccessLogWrap(hand http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path)
		hand.ServeHTTP(w, r)
	})
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

func (s *UserServiceImp) InsertBankAccount(userId int, bankAccount *BankAccount) error {
	// check user_id exist
	user, err := s.GetByID(userId)
	if err != nil {
		return err
	}

	// check account_number exist

	now := time.Now()
	bankAccount.CreatedAt = now
	bankAccount.UpdatedAt = now
	row := s.db.QueryRow("INSERT INTO bank_accounts (user_id, account_number, account_name, created_at, updated_at) values ($1, $2, $3, $4, $5) RETURNING id", user.ID, bankAccount.AccountNumber, user.FirstName+" "+user.LastName, now, now)

	if err := row.Scan(&user.ID); err != nil {
		return err
	}
	return nil
}

func (s *UserServiceImp) GetBankAccountByUserId(id int) (*BankAccount, error) {
	stmt := "SELECT * FROM bank_account WHERE user_id = $1"
	row := s.db.QueryRow(stmt, id)
	var bankAccount BankAccount
	err := row.Scan(&bankAccount.ID, &bankAccount.UserID, &bankAccount.AccountNumber, &bankAccount.AccountNumber, &bankAccount.Balance, &bankAccount.CreatedAt, &bankAccount.UpdatedAt, &bankAccount.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &bankAccount, nil
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

	if err := s.userService.InsertBankAccount(userId, &bankAccount); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, bankAccount)
}

func (s *Server) GetBankAccountByUserId(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	user, err := s.userService.GetBankAccountByUserId(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func setupRoute(s *Server) *gin.Engine {
	r := gin.New()
	r.Use(RequestLogger())
	users := r.Group("/users")
	//_ := r.Group("/bankAccount")
	//admin := r.Group("/admin")

	//admin.Use(gin.BasicAuth(gin.Accounts{
	//	"admin": "1234",
	//}))
	//users.Use(s.AuthTodo)
	users.GET("/", s.All)
	users.POST("/", s.Create)
	users.GET("/:id", s.GetByID)
	users.PUT("/:id", s.Update)
	users.DELETE("/:id", s.DeleteByID)

	users.POST("/:id/bankAccount", s.CreateBankAccount)
	users.GET("/:id/bankAccount", s.GetBankAccountByUserId)
	//admin.POST("/secrets", s.CreateSecret)

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
		user_id SERIAL,
		account_number TEXT UNIQUE,
		account_name TEXT,
		balance TEXT,
		created_at TIMESTAMP WITHOUT TIME ZONE,
		updated_at TIMESTAMP WITHOUT TIME ZONE
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
		//bankAccountService: &BankAccountServiceImp{
		//	db: db,
		//},
	}

	r := setupRoute(s)

	r.Run(":" + os.Getenv("PORT"))
}
